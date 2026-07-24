package metal

import (
	"fmt"
	"log/slog"
	"strings"
	"syscall"
	"unsafe"

	"github.com/nickheyer/greetdeez/pkg/outputs"
)

// pure-go drm/km
// struct layouts from include/uapi/drm/drm_mode.h

type drmModeInfo struct {
	Clock                                         uint32
	HDisplay, HSyncStart, HSyncEnd, HTotal, HSkew uint16
	VDisplay, VSyncStart, VSyncEnd, VTotal, VScan uint16
	VRefresh                                      uint32
	Flags                                         uint32
	Type                                          uint32
	Name                                          [32]byte
}

type drmCardRes struct {
	FbIDPtr, CrtcIDPtr, ConnectorIDPtr, EncoderIDPtr     uint64
	CountFbs, CountCrtcs, CountConnectors, CountEncoders uint32
	MinWidth, MaxWidth, MinHeight, MaxHeight             uint32
}

type drmGetConnector struct {
	EncodersPtr, ModesPtr, PropsPtr, PropValuesPtr         uint64
	CountModes, CountProps, CountEncoders                  uint32
	EncoderID, ConnectorID, ConnectorType, ConnectorTypeID uint32
	Connection                                             uint32
	MmWidth, MmHeight                                      uint32
	Subpixel                                               uint32
	Pad                                                    uint32
}

type drmGetEncoder struct {
	EncoderID, EncoderType, CrtcID, PossibleCrtcs, PossibleClones uint32
}

type drmModeCrtc struct {
	SetConnectorsPtr uint64
	CountConnectors  uint32
	CrtcID           uint32
	FbID             uint32
	X, Y             uint32
	GammaSize        uint32
	ModeValid        uint32
	Mode             drmModeInfo
}

type drmCreateDumb struct {
	Height, Width, Bpp, Flags uint32
	Handle, Pitch             uint32
	Size                      uint64
}

type drmMapDumb struct {
	Handle, Pad uint32
	Offset      uint64
}

type drmDestroyDumb struct {
	Handle uint32
}

type drmFbCmd struct {
	FbID, Width, Height, Pitch, Bpp, Depth, Handle uint32
}

type drmPageFlip struct {
	CrtcID, FbID, Flags, Reserved uint32
	UserData                      uint64
}

const (
	drmConnected        = 1
	drmModeTypePrefer   = 1 << 3
	drmPageFlipEvent    = 0x01
	drmEventFlipDone    = 0x02
	drmEventHeaderBytes = 8 // u32 type + u32 length
)

func drmIO(nr uintptr) uintptr         { return 'd'<<8 | nr }
func drmIOWR(nr, size uintptr) uintptr { return 3<<30 | size<<16 | 'd'<<8 | nr }
func ioctl(fd int, req uintptr, arg unsafe.Pointer) error {
	for {
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
		if errno == 0 {
			return nil
		}
		if errno == syscall.EINTR || errno == syscall.EAGAIN {
			continue
		}
		return errno
	}
}

var (
	ioctlSetMaster    = drmIO(0x1e)
	ioctlDropMaster   = drmIO(0x1f)
	ioctlGetResources = drmIOWR(0xA0, unsafe.Sizeof(drmCardRes{}))
	ioctlGetCrtc      = drmIOWR(0xA1, unsafe.Sizeof(drmModeCrtc{}))
	ioctlSetCrtc      = drmIOWR(0xA2, unsafe.Sizeof(drmModeCrtc{}))
	ioctlGetEncoder   = drmIOWR(0xA6, unsafe.Sizeof(drmGetEncoder{}))
	ioctlGetConnector = drmIOWR(0xA7, unsafe.Sizeof(drmGetConnector{}))
	ioctlAddFB        = drmIOWR(0xAE, unsafe.Sizeof(drmFbCmd{}))
	ioctlRmFB         = drmIOWR(0xAF, 4)
	ioctlPageFlip     = drmIOWR(0xB0, unsafe.Sizeof(drmPageFlip{}))
	ioctlCreateDumb   = drmIOWR(0xB2, unsafe.Sizeof(drmCreateDumb{}))
	ioctlMapDumb      = drmIOWR(0xB3, unsafe.Sizeof(drmMapDumb{}))
	ioctlDestroyDumb  = drmIOWR(0xB4, unsafe.Sizeof(drmDestroyDumb{}))
)

type dumbFB struct {
	handle uint32
	pitch  uint32
	size   uint64
	fbID   uint32
	mem    []byte
}

// Owns one connector one crtc and two dumb buffers
type DRMSurface struct {
	fd        int
	connector uint32
	crtc      uint32
	mode      drmModeInfo
	fbs       [2]dumbFB
	back      int
	flipped   bool        // first present uses setcrtc
	noFlip    bool        // driver refused page flip fall back to setcrtc
	saved     []crtcState // every active crtc as found, put back on Close
	otherFds  []int       // extra cards held open so blanked outputs stay dark
}

// pre-greeter state of one crtc and the connectors it was driving
type crtcState struct {
	fd    int
	crtc  drmModeCrtc
	conns []uint32
}

// connector type names from drm_connector_enum_list in the kernel
var connectorTypeNames = [...]string{
	"Unknown", "VGA", "DVI-I", "DVI-D", "DVI-A", "Composite", "SVIDEO",
	"LVDS", "Component", "DIN", "DP", "HDMI-A", "HDMI-B", "TV", "eDP",
	"Virtual", "DSI", "DPI", "Writeback", "SPI", "USB",
}

func connectorName(typ, id uint32) string {
	if int(typ) < len(connectorTypeNames) {
		return fmt.Sprintf("%s-%d", connectorTypeNames[typ], id)
	}
	return fmt.Sprintf("Unknown%d-%d", typ, id)
}

// one connected connector that could host the greeter
type drmCandidate struct {
	fd       int
	card     string
	connID   uint32
	name     string
	conn     drmGetConnector
	mode     drmModeInfo
	encoders []uint32
	crtcs    []uint32
}

// OpenDRM enumerates connected connectors on every card and takes over the
// one selected by the shared output policy. want is the configured connector
// name ("DP-1", "HDMI-A-1", ...), empty for auto.
func OpenDRM(want string) (*DRMSurface, error) {
	cands, fds, err := enumerateOutputs()
	if err != nil {
		return nil, err
	}
	// the retry loop below compacts cands in place keep a copy so restore
	// still knows about every connector we saw
	all := append([]drmCandidate(nil), cands...)

	outs := make([]outputs.Output, len(cands))
	for i, c := range cands {
		outs[i] = outputs.Output{
			Name: c.name, Width: int(c.mode.HDisplay), Height: int(c.mode.VDisplay),
			WidthMM: int(c.conn.MmWidth), HeightMM: int(c.conn.MmHeight),
		}
		slog.Info("drm: output", "card", c.card, "connector", c.name,
			"mode", fmt.Sprintf("%dx%d@%d", c.mode.HDisplay, c.mode.VDisplay, c.mode.VRefresh),
			"physical_mm", fmt.Sprintf("%dx%d", c.conn.MmWidth, c.conn.MmHeight))
	}
	if want != "" && !strings.EqualFold(cands[outputs.Pick(outs, want)].name, want) {
		slog.Warn("drm: configured output not connected, using auto", "want", want)
	}

	var lastErr error
	for len(cands) > 0 {
		idx := outputs.Pick(outs, want)
		c := cands[idx]
		s, err := setupOutput(c)
		if err == nil {
			s.captureAndBlank(all, fds)
			slog.Info("drm: output ready", "card", c.card, "connector", c.name,
				"mode", fmt.Sprintf("%dx%d@%d", s.mode.HDisplay, s.mode.VDisplay, s.mode.VRefresh))
			return s, nil
		}
		lastErr = fmt.Errorf("%s %s: %w", c.card, c.name, err)
		slog.Warn("drm: output unusable, trying next", "connector", c.name, "error", err)
		cands = append(cands[:idx], cands[idx+1:]...)
		outs = append(outs[:idx], outs[idx+1:]...)
		want = ""
	}
	for _, fd := range fds {
		syscall.Close(fd)
	}
	return nil, lastErr
}

// enumerateOutputs opens every card and lists connected connectors with
// modes. Returned fds are the open card fds backing the candidates; the
// caller owns them.
func enumerateOutputs() ([]drmCandidate, []int, error) {
	var cands []drmCandidate
	var fds []int
	var lastErr error
	for i := 0; i < 8; i++ {
		path := fmt.Sprintf("/dev/dri/card%d", i)
		fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_CLOEXEC, 0)
		if err != nil {
			if lastErr == nil {
				lastErr = fmt.Errorf("open %s: %w", path, err)
			}
			continue
		}
		// best effort another master means setcrtc will fail below anyway
		if err := ioctl(fd, ioctlSetMaster, nil); err != nil {
			slog.Debug("drm: set_master", "error", err)
		}
		_, crtcs, connectors, err := getResources(fd)
		if err != nil {
			lastErr = fmt.Errorf("%s: get resources: %w", path, err)
			syscall.Close(fd)
			continue
		}
		found := false
		for _, connID := range connectors {
			conn, modes, encoders, err := getConnector(fd, connID)
			if err != nil || conn.Connection != drmConnected || len(modes) == 0 {
				continue
			}
			cands = append(cands, drmCandidate{
				fd: fd, card: path, connID: connID,
				name:     connectorName(conn.ConnectorType, conn.ConnectorTypeID),
				conn:     conn,
				mode:     pickMode(modes),
				encoders: encoders,
				crtcs:    crtcs,
			})
			found = true
		}
		if !found {
			if lastErr == nil {
				lastErr = fmt.Errorf("%s: no connected connector with modes", path)
			}
			syscall.Close(fd)
			continue
		}
		fds = append(fds, fd)
	}
	if len(cands) == 0 {
		if lastErr == nil {
			lastErr = fmt.Errorf("no /dev/dri/card* devices (is the greeter user in the video group?)")
		}
		return nil, nil, lastErr
	}
	return cands, fds, nil
}

func setupOutput(c drmCandidate) (*DRMSurface, error) {
	crtc, err := pickCrtc(c.fd, c.conn, c.encoders, c.crtcs)
	if err != nil {
		return nil, err
	}

	s := &DRMSurface{fd: c.fd, connector: c.connID, crtc: crtc, mode: c.mode}

	if err := s.createBuffers(); err != nil {
		return nil, err
	}
	return s, nil
}

// currentCrtcOf reports the crtc a connector is routed to right now, 0 if dark.
func currentCrtcOf(fd int, conn drmGetConnector) uint32 {
	if conn.EncoderID == 0 {
		return 0
	}
	enc := drmGetEncoder{EncoderID: conn.EncoderID}
	if err := ioctl(fd, ioctlGetEncoder, unsafe.Pointer(&enc)); err != nil {
		return 0
	}
	return enc.CrtcID
}

// Cap active crtc so Close can hand the console back as it found it
func (s *DRMSurface) captureAndBlank(all []drmCandidate, fds []int) {
	crtcsByFd := make(map[int][]uint32)
	routes := make(map[int]map[uint32][]uint32)
	for _, c := range all {
		crtcsByFd[c.fd] = c.crtcs
		if id := currentCrtcOf(c.fd, c.conn); id != 0 {
			if routes[c.fd] == nil {
				routes[c.fd] = make(map[uint32][]uint32)
			}
			routes[c.fd][id] = append(routes[c.fd][id], c.connID)
		}
	}
	for fd, crtcs := range crtcsByFd {
		for _, id := range crtcs {
			st := crtcState{fd: fd, conns: routes[fd][id]}
			st.crtc.CrtcID = id
			if err := ioctl(fd, ioctlGetCrtc, unsafe.Pointer(&st.crtc)); err != nil {
				continue
			}
			if st.crtc.ModeValid == 0 && st.crtc.FbID == 0 {
				continue // already dark nothing to restore
			}
			s.saved = append(s.saved, st)
			if fd == s.fd && id == s.crtc {
				continue // ours the first present takes it over
			}
			off := drmModeCrtc{CrtcID: id}
			if err := ioctl(fd, ioctlSetCrtc, unsafe.Pointer(&off)); err != nil {
				slog.Debug("drm: blank crtc", "crtc", id, "error", err)
			}
		}
	}
	for _, fd := range fds {
		if fd != s.fd {
			s.otherFds = append(s.otherFds, fd)
		}
	}
}

func getResources(fd int) (drmCardRes, []uint32, []uint32, error) {
	for attempt := 0; attempt < 4; attempt++ {
		var res drmCardRes
		if err := ioctl(fd, ioctlGetResources, unsafe.Pointer(&res)); err != nil {
			return res, nil, nil, err
		}
		crtcs := make([]uint32, max(res.CountCrtcs, 1))
		conns := make([]uint32, max(res.CountConnectors, 1))
		fbs := make([]uint32, max(res.CountFbs, 1))
		encs := make([]uint32, max(res.CountEncoders, 1))
		want := res
		res.CrtcIDPtr = uint64(uintptr(unsafe.Pointer(&crtcs[0])))
		res.ConnectorIDPtr = uint64(uintptr(unsafe.Pointer(&conns[0])))
		res.FbIDPtr = uint64(uintptr(unsafe.Pointer(&fbs[0])))
		res.EncoderIDPtr = uint64(uintptr(unsafe.Pointer(&encs[0])))
		if err := ioctl(fd, ioctlGetResources, unsafe.Pointer(&res)); err != nil {
			return res, nil, nil, err
		}
		// hotplug between calls retry
		if res.CountCrtcs > want.CountCrtcs || res.CountConnectors > want.CountConnectors {
			continue
		}
		return res, crtcs[:res.CountCrtcs], conns[:res.CountConnectors], nil
	}
	return drmCardRes{}, nil, nil, fmt.Errorf("resources kept changing")
}

func getConnector(fd int, id uint32) (drmGetConnector, []drmModeInfo, []uint32, error) {
	for attempt := 0; attempt < 4; attempt++ {
		conn := drmGetConnector{ConnectorID: id}
		if err := ioctl(fd, ioctlGetConnector, unsafe.Pointer(&conn)); err != nil {
			return conn, nil, nil, err
		}
		modes := make([]drmModeInfo, max(conn.CountModes, 1))
		encs := make([]uint32, max(conn.CountEncoders, 1))
		want := conn
		conn = drmGetConnector{ConnectorID: id,
			CountModes:    want.CountModes,
			CountEncoders: want.CountEncoders,
			ModesPtr:      uint64(uintptr(unsafe.Pointer(&modes[0]))),
			EncodersPtr:   uint64(uintptr(unsafe.Pointer(&encs[0]))),
		}
		if err := ioctl(fd, ioctlGetConnector, unsafe.Pointer(&conn)); err != nil {
			return conn, nil, nil, err
		}
		if conn.CountModes > want.CountModes || conn.CountEncoders > want.CountEncoders {
			continue
		}
		return conn, modes[:conn.CountModes], encs[:conn.CountEncoders], nil
	}
	return drmGetConnector{}, nil, nil, fmt.Errorf("connector kept changing")
}

func pickMode(modes []drmModeInfo) drmModeInfo {
	for _, m := range modes {
		if m.Type&drmModeTypePrefer != 0 {
			return m
		}
	}
	return modes[0]
}

func pickCrtc(fd int, conn drmGetConnector, encoders, crtcs []uint32) (uint32, error) {
	// current encoder first
	if conn.EncoderID != 0 {
		enc := drmGetEncoder{EncoderID: conn.EncoderID}
		if err := ioctl(fd, ioctlGetEncoder, unsafe.Pointer(&enc)); err == nil && enc.CrtcID != 0 {
			return enc.CrtcID, nil
		}
	}
	// otherwise first crtc an encoder can drive
	for _, encID := range encoders {
		enc := drmGetEncoder{EncoderID: encID}
		if err := ioctl(fd, ioctlGetEncoder, unsafe.Pointer(&enc)); err != nil {
			continue
		}
		for i, crtcID := range crtcs {
			if enc.PossibleCrtcs&(1<<uint(i)) != 0 {
				return crtcID, nil
			}
		}
	}
	if len(crtcs) > 0 {
		return crtcs[0], nil
	}
	return 0, fmt.Errorf("no usable crtc")
}

func (s *DRMSurface) createBuffers() error {
	w, h := uint32(s.mode.HDisplay), uint32(s.mode.VDisplay)
	for i := range s.fbs {
		create := drmCreateDumb{Width: w, Height: h, Bpp: 32}
		if err := ioctl(s.fd, ioctlCreateDumb, unsafe.Pointer(&create)); err != nil {
			return fmt.Errorf("create dumb buffer: %w", err)
		}
		fb := dumbFB{handle: create.Handle, pitch: create.Pitch, size: create.Size}

		cmd := drmFbCmd{Width: w, Height: h, Pitch: create.Pitch, Bpp: 32, Depth: 24, Handle: create.Handle}
		if err := ioctl(s.fd, ioctlAddFB, unsafe.Pointer(&cmd)); err != nil {
			return fmt.Errorf("addfb: %w", err)
		}
		fb.fbID = cmd.FbID

		mreq := drmMapDumb{Handle: create.Handle}
		if err := ioctl(s.fd, ioctlMapDumb, unsafe.Pointer(&mreq)); err != nil {
			return fmt.Errorf("map dumb buffer: %w", err)
		}
		mem, err := syscall.Mmap(s.fd, int64(mreq.Offset), int(create.Size),
			syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
		if err != nil {
			return fmt.Errorf("mmap fb: %w", err)
		}
		fb.mem = mem
		s.fbs[i] = fb
	}
	return nil
}

func (s *DRMSurface) Size() (int, int) {
	return int(s.mode.HDisplay), int(s.mode.VDisplay)
}

// Present copies the frame into the back buffer and flips at vsync.
func (s *DRMSurface) Present(f *Frame) error {
	fb := &s.fbs[s.back]
	copyFrameToFB(f, fb, int(s.mode.HDisplay), int(s.mode.VDisplay))

	if !s.flipped {
		if err := s.setCrtc(fb.fbID); err != nil {
			return fmt.Errorf("setcrtc: %w", err)
		}
		s.flipped = true
		s.back ^= 1
		return nil
	}

	if s.noFlip {
		if err := s.setCrtc(fb.fbID); err != nil {
			return err
		}
		s.back ^= 1
		return nil
	}

	flip := drmPageFlip{CrtcID: s.crtc, FbID: fb.fbID, Flags: drmPageFlipEvent}
	if err := ioctl(s.fd, ioctlPageFlip, unsafe.Pointer(&flip)); err != nil {
		// virtio and friends occasionally lack async flips
		slog.Warn("drm: page flip unavailable, falling back to setcrtc", "error", err)
		s.noFlip = true
		if err := s.setCrtc(fb.fbID); err != nil {
			return err
		}
		s.back ^= 1
		return nil
	}
	if err := s.waitFlip(); err != nil {
		return err
	}
	s.back ^= 1
	return nil
}

func (s *DRMSurface) setCrtc(fbID uint32) error {
	conn := s.connector
	crtc := drmModeCrtc{
		SetConnectorsPtr: uint64(uintptr(unsafe.Pointer(&conn))),
		CountConnectors:  1,
		CrtcID:           s.crtc,
		FbID:             fbID,
		ModeValid:        1,
		Mode:             s.mode,
	}
	return ioctl(s.fd, ioctlSetCrtc, unsafe.Pointer(&crtc))
}

// Blocks until the kernel reports the flip completed
func (s *DRMSurface) waitFlip() error {
	buf := make([]byte, 1024)
	for {
		n, err := syscall.Read(s.fd, buf)
		if err != nil {
			if err == syscall.EINTR || err == syscall.EAGAIN {
				continue
			}
			return fmt.Errorf("read drm events: %w", err)
		}
		for off := 0; off+drmEventHeaderBytes <= n; {
			typ := *(*uint32)(unsafe.Pointer(&buf[off]))
			length := *(*uint32)(unsafe.Pointer(&buf[off+4]))
			if length < drmEventHeaderBytes || off+int(length) > n {
				break
			}
			if typ == drmEventFlipDone {
				return nil
			}
			off += int(length)
		}
	}
}

func copyFrameToFB(f *Frame, fb *dumbFB, w, h int) {
	src := unsafe.Slice((*byte)(unsafe.Pointer(&f.Pix[0])), len(f.Pix)*4)
	rowBytes := w * 4
	parallelRows(h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			copy(fb.mem[y*int(fb.pitch):y*int(fb.pitch)+rowBytes], src[y*rowBytes:(y+1)*rowBytes])
		}
	})
}

func (s *DRMSurface) Close() {
	// hand every crtc back before dropping our fbs
	for _, st := range s.saved {
		set := drmModeCrtc{CrtcID: st.crtc.CrtcID}
		if st.crtc.ModeValid != 0 && st.crtc.FbID != 0 && len(st.conns) > 0 {
			set = st.crtc
			set.SetConnectorsPtr = uint64(uintptr(unsafe.Pointer(&st.conns[0])))
			set.CountConnectors = uint32(len(st.conns))
		}
		if err := ioctl(st.fd, ioctlSetCrtc, unsafe.Pointer(&set)); err != nil {
			slog.Debug("drm: restore crtc", "crtc", st.crtc.CrtcID, "error", err)
		}
	}
	for i := range s.fbs {
		fb := &s.fbs[i]
		if fb.mem != nil {
			syscall.Munmap(fb.mem)
		}
		if fb.fbID != 0 {
			id := fb.fbID
			ioctl(s.fd, ioctlRmFB, unsafe.Pointer(&id))
		}
		if fb.handle != 0 {
			d := drmDestroyDumb{Handle: fb.handle}
			ioctl(s.fd, ioctlDestroyDumb, unsafe.Pointer(&d))
		}
	}
	for _, fd := range s.otherFds {
		ioctl(fd, ioctlDropMaster, nil)
		syscall.Close(fd)
	}
	ioctl(s.fd, ioctlDropMaster, nil)
	syscall.Close(s.fd)
}
