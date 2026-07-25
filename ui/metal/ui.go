package metal

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	pb "github.com/nickheyer/greetdeez/gen/go/greetdeez/v1"
)

type phase int

const (
	phaseUser phase = iota
	phasePrompt
	phaseBusy
	phaseWarp
)

const (
	bootDur  = 0.8
	warpDur  = 0.75
	errorDur = 2.8
	armDur   = 2.5

	maxUserLen  = 48
	maxInputLen = 128
)

// palette
var (
	colAccent = rgb(0x2e, 0xd4, 0xc3)
	colEmber  = rgb(0xff, 0x8a, 0x3d)
	colText   = rgb(0xe8, 0xf0, 0xf2)
	colDim    = rgb(0x5a, 0x6a, 0x78)
	colPanel  = rgb(0x07, 0x0b, 0x12)
	colField  = rgb(0x0c, 0x14, 0x1e)
	colError  = rgb(0xff, 0x3b, 0x4b)
	colAmber  = rgb(0xff, 0xc4, 0x5e)
	colShadow = rgb(0x02, 0x03, 0x06)
)

type authResult struct {
	step *pb.AuthStep
	err  error
}

type startResult struct{ err error }
type powerResult struct{ err error }

// UI is the whole login screen: effects, state machine, renderer.
type UI struct {
	be   Backend
	w, h int

	// effects
	plasma   *Plasma
	stars    *Starfield
	scroller *Scroller
	crt      *CRT

	// data from the backend
	hostname string
	sessions []*pb.Session
	sessIdx  int
	caps     *pb.PowerCapabilities

	// state machine
	phase    phase
	username []rune
	input    []rune
	focusPw  bool // phaseUser: password field focused instead of login
	prompt   *pb.AuthPrompt
	msgs     []string
	busyMsg  string
	errMsg   string
	errAt    float64
	warpAt   float64
	powerArm pb.PowerAction
	armAt    float64
	mods     Mods
	resCh    chan any

	// pointer
	mx, my float64
	mSeen  bool
	mDown  bool
	trail  []trailPt

	// clickable regions captured during render, hit-tested next frame
	hitUser   rect
	hitPrompt rect
	hitSess   rect

	// exit
	Done    bool
	Success bool

	// dev mode: esc at the login screen quits, nothing to log into
	Dev bool

	// wall clock injected for deterministic snapshots
	Clock func() time.Time

	// font scales derived from resolution
	sS, sM, sL int
}

func NewUI(be Backend, w, h int) *UI {
	u := &UI{
		be:     be,
		w:      w,
		h:      h,
		plasma: NewPlasma((w+1)/2, (h+1)/2),
		stars:  NewStarfield(w, h, 420),
		crt:    NewCRT(w, h),
		resCh:  make(chan any, 4),
		Clock:  time.Now,
	}
	u.sS = max(1, h/360)
	u.sM = u.sS
	u.sL = u.sS * 2
	u.scroller = NewScroller(txtMarquee, u.sS)

	u.loadBackendData()
	return u
}

func (u *UI) loadBackendData() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	u.hostname = txtFallbackHostname
	if info, err := u.be.SystemInfo(ctx); err == nil && info.GetHostname() != "" {
		u.hostname = info.GetHostname()
	}
	if sessions, err := u.be.Sessions(ctx); err == nil {
		u.sessions = sessions
	}
	if len(u.sessions) == 0 {
		// still let people in on a bare box
		u.sessions = []*pb.Session{{Name: txtFallbackSession, Cmd: []string{"/bin/sh", "-l"}}}
	}
	if caps, err := u.be.PowerCaps(ctx); err == nil {
		u.caps = caps
	} else {
		u.caps = &pb.PowerCapabilities{}
	}
	if st, err := u.be.State(ctx); err == nil && st != nil {
		u.username = []rune(st.GetLastUser())
		match := -1
		for i, s := range u.sessions {
			if s.GetName() != st.GetLastSession() {
				continue
			}
			if s.GetType() == st.GetLastSessionType() {
				match = i
				break
			}
			if match < 0 {
				match = i
			}
		}
		if match >= 0 {
			u.sessIdx = match
		}
	}
}

func (u *UI) session() *pb.Session { return u.sessions[u.sessIdx] }

func (u *UI) cycleSession(dir int) {
	if len(u.sessions) > 1 {
		u.sessIdx = (u.sessIdx + dir + len(u.sessions)) % len(u.sessions)
	}
}

// backToUser abandons the pam conversation and refocuses the login field.
func (u *UI) backToUser() {
	go u.be.CancelAuth(context.Background()) //nolint:errcheck
	u.input = u.input[:0]
	u.prompt = nil
	u.focusPw = false
	u.phase = phaseUser
}

// ── input ───────────────────────────────────────────────────────

// Keyboard model: both form fields are local, tab and clicks only move
// focus, arrows change the session. PAM is spoken to exactly once, on
// enter, so stray tabs and clicks can never burn faillock attempts.
func (u *UI) HandleKey(ev KeyEvent, now float64) {
	if u.mods.Track(ev.Code, ev.Down) || !ev.Down {
		return
	}

	if u.handlePowerKey(ev.Code, now) {
		return
	}

	switch u.phase {
	case phaseUser:
		switch ev.Code {
		case keyEnter, keyKpEnter:
			u.beginAuth(now)
		case keyTab:
			u.focusPw = !u.focusPw
		case keyEsc:
			if u.Dev {
				u.Done = true
			}
		case keyBackspace:
			buf := &u.username
			if u.focusPw {
				buf = &u.input
			}
			if u.mods.Ctrl() {
				*buf = (*buf)[:0]
			} else if len(*buf) > 0 {
				*buf = (*buf)[:len(*buf)-1]
			}
		case keyRight, keyDown:
			u.cycleSession(1)
		case keyLeft, keyUp:
			u.cycleSession(-1)
		default:
			r := u.mods.Rune(ev.Code)
			if r == 0 {
				break
			}
			if u.focusPw {
				if len(u.input) < maxInputLen {
					u.input = append(u.input, r)
				}
			} else if len(u.username) < maxUserLen {
				u.username = append(u.username, r)
			}
		}

	case phasePrompt:
		switch ev.Code {
		case keyEnter, keyKpEnter:
			u.respondAuth(now)
		case keyTab, keyEsc:
			// two fields so tab wraps back to login
			u.backToUser()
		case keyBackspace:
			if u.mods.Ctrl() {
				u.input = u.input[:0]
			} else if len(u.input) > 0 {
				u.input = u.input[:len(u.input)-1]
			}
		case keyRight, keyDown:
			u.cycleSession(1)
		case keyLeft, keyUp:
			u.cycleSession(-1)
		default:
			if r := u.mods.Rune(ev.Code); r != 0 && len(u.input) < maxInputLen {
				u.input = append(u.input, r)
			}
		}
	}
}

// ResetMods drops modifier state for presenters that lose key releases,
// like a vt switch away with ctrl+alt still held.
func (u *UI) ResetMods() { u.mods = Mods{} }

// ── mouse ───────────────────────────────────────────────────────

type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

type trailPt struct {
	x, y float64
	t    float64
}

// HandleMouse tracks the pointer, stirs the lava, and hit-tests clicks
// against the regions captured on the previous frame.
func (u *UI) HandleMouse(ev MouseEvent, now float64) {
	var dx, dy float64
	if ev.Abs {
		dx, dy = ev.X-u.mx, ev.Y-u.my
		u.mx, u.my = ev.X, ev.Y
	} else {
		dx, dy = ev.DX, ev.DY
		u.mx += dx
		u.my += dy
	}
	u.mx = math.Max(0, math.Min(float64(u.w-1), u.mx))
	u.my = math.Max(0, math.Min(float64(u.h-1), u.my))

	first := !u.mSeen
	u.mSeen = true
	if dx != 0 || dy != 0 {
		speed := math.Hypot(dx, dy)
		if first {
			speed = 0 // a jump to the initial position is not motion
		}
		u.plasma.SetPointer(u.mx/2, u.my/2, speed/2)
		u.trail = append(u.trail, trailPt{u.mx, u.my, now})
		if len(u.trail) > 32 {
			u.trail = u.trail[len(u.trail)-32:]
		}
	}

	if ev.Wheel != 0 && (u.phase == phaseUser || u.phase == phasePrompt) {
		// wheel down walks forward through the session list
		u.cycleSession(-ev.Wheel)
	}

	if ev.Btn != 0 {
		if ev.Btn == 1 {
			u.mDown = ev.Down
		}
		if ev.Down {
			u.plasma.Pulse()
			if ev.Btn == 1 {
				u.click(int(u.mx), int(u.my))
			}
		}
	}
}

func (u *UI) click(x, y int) {
	switch {
	case u.hitSess.contains(x, y) && (u.phase == phaseUser || u.phase == phasePrompt):
		if x < u.hitSess.x+u.hitSess.w/2 {
			u.cycleSession(-1)
		} else {
			u.cycleSession(1)
		}
	case u.phase == phasePrompt && u.hitUser.contains(x, y):
		u.backToUser()
	case u.phase == phaseUser && u.hitUser.contains(x, y):
		u.focusPw = false
	case u.phase == phaseUser && u.hitPrompt.contains(x, y):
		u.focusPw = true
	}
}

func (u *UI) handlePowerKey(code uint16, now float64) bool {
	var action pb.PowerAction
	switch {
	case code == keyF10 && u.caps.GetCanPoweroff():
		action = pb.PowerAction_POWER_ACTION_POWEROFF
	case code == keyF11 && u.caps.GetCanReboot():
		action = pb.PowerAction_POWER_ACTION_REBOOT
	case code == keyF12 && u.caps.GetCanSuspend():
		action = pb.PowerAction_POWER_ACTION_SUSPEND
	default:
		return false
	}

	if u.powerArm == action && now-u.armAt < armDur {
		u.powerArm = pb.PowerAction_POWER_ACTION_UNSPECIFIED
		u.busyMsg = powerVerb(action)
		u.phase = phaseBusy
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			u.resCh <- powerResult{err: u.be.Power(ctx, action)}
		}()
		return true
	}
	u.powerArm = action
	u.armAt = now
	return true
}

func powerVerb(a pb.PowerAction) string {
	switch a {
	case pb.PowerAction_POWER_ACTION_POWEROFF:
		return txtPoweringOff
	case pb.PowerAction_POWER_ACTION_REBOOT:
		return txtRebooting
	default:
		return txtSuspending
	}
}

func powerKeyName(a pb.PowerAction) string {
	switch a {
	case pb.PowerAction_POWER_ACTION_POWEROFF:
		return txtKeyPoweroff
	case pb.PowerAction_POWER_ACTION_REBOOT:
		return txtKeyReboot
	default:
		return txtKeySuspend
	}
}

// ── auth flow ───────────────────────────────────────────────────

// beginAuth submits the whole form as one pam conversation. A filled
// password answers the first secret prompt right away; anything else
// pam wants (2fa, an empty password field) drops to the interactive
// prompt via onAuthStep.
func (u *UI) beginAuth(now float64) {
	u.phase = phaseBusy
	u.busyMsg = txtAuthenticating
	u.errMsg = ""
	user := string(u.username)
	pw := string(u.input)
	u.input = u.input[:0]
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		step, err := u.be.BeginAuth(ctx, user)
		if err == nil && pw != "" && step.GetPrompt().GetType() == pb.PromptType_PROMPT_TYPE_SECRET {
			next, nerr := u.be.RespondAuth(ctx, pw)
			if nerr == nil && next != nil {
				next.Messages = append(step.GetMessages(), next.GetMessages()...)
			}
			step, err = next, nerr
		}
		u.resCh <- authResult{step: step, err: err}
	}()
	_ = now
}

func (u *UI) respondAuth(now float64) {
	u.phase = phaseBusy
	u.busyMsg = txtAuthenticating
	resp := string(u.input)
	u.input = u.input[:0]
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		step, err := u.be.RespondAuth(ctx, resp)
		u.resCh <- authResult{step: step, err: err}
	}()
	_ = now
}

func (u *UI) startSession(now float64) {
	u.phase = phaseBusy
	sess := u.session()
	u.busyMsg = txtLaunching + strings.ToUpper(sess.GetName())
	user := string(u.username)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := u.be.StartSession(ctx, sess); err != nil {
			u.resCh <- startResult{err: err}
			return
		}
		// best effort greeter is about to die anyway
		u.be.SaveState(ctx, user, sess) //nolint:errcheck
		u.resCh <- startResult{}
	}()
	_ = now
}

func (u *UI) fail(msg string, now float64) {
	if msg == "" {
		msg = txtAccessDenied
	}
	u.errMsg = strings.ToUpper(msg)
	u.errAt = now
	u.prompt = nil
	u.input = u.input[:0]
	// focus the password for the retry the username usually survives
	u.focusPw = len(u.username) > 0
	u.phase = phaseUser
}

func (u *UI) onAuthStep(res authResult, now float64) {
	if res.err != nil {
		u.fail(res.err.Error(), now)
		return
	}
	step := res.step
	for _, m := range step.GetMessages() {
		u.msgs = append(u.msgs, m)
	}
	if len(u.msgs) > 3 {
		u.msgs = u.msgs[len(u.msgs)-3:]
	}
	switch {
	case step.GetError() != "":
		u.fail(step.GetError(), now)
	case step.GetSuccess():
		u.startSession(now)
	case step.GetPrompt() != nil:
		u.prompt = step.GetPrompt()
		u.input = u.input[:0]
		u.phase = phasePrompt
	default:
		u.fail(txtAuthFailed, now)
	}
}

// ── update ──────────────────────────────────────────────────────

func (u *UI) Update(dt, now float64) {
	for {
		select {
		case res := <-u.resCh:
			switch r := res.(type) {
			case authResult:
				u.onAuthStep(r, now)
			case startResult:
				if r.err != nil {
					u.fail(r.err.Error(), now)
				} else {
					u.phase = phaseWarp
					u.warpAt = now
				}
			case powerResult:
				if r.err != nil {
					u.fail(r.err.Error(), now)
				}
				// on success the machine is going away no state change
			}
		default:
			goto drained
		}
	}
drained:

	if u.phase == phaseWarp {
		k := (now - u.warpAt) / warpDur
		u.stars.SetWarp(float32(k))
		if k >= 1 {
			u.Done = true
			u.Success = true
		}
	}

	u.plasma.Update(dt)
	u.stars.Update(float32(dt))
	u.scroller.Update(dt, u.w)
}

// ── render ──────────────────────────────────────────────────────

func (u *UI) Render(f *Frame, now float64) {
	u.plasma.Render(f, now*0.35)
	u.stars.Render(f)

	if u.phase != phaseWarp {
		u.scroller.Render(f, u.h-14*u.sS, now)
		u.renderClock(f, now)
		u.renderPanel(f, now)
	}

	u.crt.Apply(f)
	u.renderOverlays(f, now)

	if u.mSeen && u.phase != phaseWarp {
		u.renderCursor(f, now)
	}
}

// renderCursor draws the pointer: a comet trail, an ember glow, and a
// sharp delta-wing arrow that tightens while the button is held.
func (u *UI) renderCursor(f *Frame, now float64) {
	// drop trail points older than the fade window
	const fade = 0.35
	for len(u.trail) > 0 && now-u.trail[0].t > fade {
		u.trail = u.trail[1:]
	}
	for _, p := range u.trail {
		age := (now - p.t) / fade
		a := uint32(90 * (1 - age))
		r := max(1, int(float64(u.sM)*2*(1-age)))
		f.CircleAdd(int(p.x), int(p.y), r, dim(colAccent, a))
	}

	mx, my := int(u.mx), int(u.my)

	// warm halo so the pointer feels like part of the lava
	f.CircleAdd(mx, my, 5*u.sM, dim(colEmber, 26))
	f.CircleAdd(mx, my, 2*u.sM, dim(colAccent, 60))

	// delta-wing: tip at the hotspot, swept trailing edge
	s := u.sM
	if u.mDown {
		s = max(1, s*3/4)
	}
	x1, y1 := mx, my+11*s // straight edge down
	x2, y2 := mx+8*s, my+8*s
	f.FillTri(mx+1, my+1, x1+1, y1+1, x2+1, y2+1, colShadow)
	f.FillTri(mx, my, x1, y1, x2, y2, colPanel)
	// edge light: accent on the leading edges ember spark at the tip
	drawLineAdd(f, mx, my, x1, y1, colAccent)
	drawLineAdd(f, mx, my, x2, y2, colAccent)
	drawLineAdd(f, x1, y1, x2, y2, dim(colAccent, 140))
	pulse := uint32(150 + 100*math.Sin(now*5))
	f.Add(mx, my, dim(colEmber, pulse))
	f.Add(mx+1, my, dim(colEmber, pulse/2))
	f.Add(mx, my+1, dim(colEmber, pulse/2))
}

func (u *UI) renderClock(f *Frame, now float64) {
	t := u.Clock()
	clock := t.Format(fmtClockTime)
	date := strings.ToUpper(t.Format(fmtClockDate))
	cw := TextWidth(clock, u.sL)
	x := (u.w - cw) / 2
	y := u.h / 14
	f.DrawTextShadow(x, y, clock, u.sL, colText, colShadow)
	dw := TextWidth(date, u.sS)
	f.DrawTextShadow((u.w-dw)/2, y+9*u.sL, date, u.sS, colDim, colShadow)
	_ = now
}

// all vertical sizes in sM units so every resolution lays out the same
const (
	luPadTop  = 6
	luHost    = 16 // 8 * sL where sL = 2*sM
	luHostGap = 4
	luSub     = 8
	luSubGap  = 5
	luField   = 24 // label 8 + gap 1 + box 12 + gap 3
	luSessGap = 2
	luSess    = 8
	luMsgGap  = 4
	luMsg     = 9 // line 8 + gap 1
	luStatus  = 8
	luPadBot  = 6
	luTotal   = luPadTop + luHost + luHostGap + luSub + luSubGap + 2*luField + luSessGap + luSess + luMsgGap + 2*luMsg + luStatus + luPadBot
)

func (u *UI) panelRect() (x, y, w, h int) {
	ch := glyphW * u.sM // field char width
	w = min(ch*30+12*u.sM, u.w-4*u.sM)
	h = luTotal * u.sM
	x = (u.w - w) / 2
	y = (u.h - h) / 2
	return
}

func (u *UI) renderPanel(f *Frame, now float64) {
	px, py, pw, ph := u.panelRect()

	// error shake
	if u.errMsg != "" && now-u.errAt < 0.4 {
		decay := 1 - (now-u.errAt)/0.4
		px += int(math.Sin(now*70) * 6 * decay * float64(u.sM))
	}

	f.BlendRect(px, py, pw, ph, colPanel, 216)
	f.Border(px, py, pw, ph, dim(colAccent, 140))
	u.corners(f, px, py, pw, ph)

	cx := px + 6*u.sM // content left
	cy := py + luPadTop*u.sM

	// hostname per char bob shrink and clip if it will not fit
	host := strings.ToUpper(u.hostname)
	hostScale := u.sL
	if TextWidth(host, hostScale) > pw-8*u.sM {
		hostScale = u.sM
	}
	maxHostChars := (pw - 8*u.sM) / (glyphW * hostScale)
	if len(host) > maxHostChars && maxHostChars > 0 {
		host = host[:maxHostChars]
	}
	hw := TextWidth(host, hostScale)
	hx := px + (pw-hw)/2
	hy := cy + (luHost*u.sM-8*hostScale)/2
	for i, ch := range host {
		bob := int(math.Sin(now*2.2+float64(i)*0.55) * float64(u.sM))
		f.drawGlyph(hx+i*glyphW*hostScale, hy+bob, glyph(ch), hostScale, colText, colAccent)
	}
	cy += (luHost + luHostGap) * u.sM

	sub := txtSubtitle
	f.DrawText(px+(pw-TextWidth(sub, u.sS))/2, cy, sub, u.sS, colDim)
	cy += (luSub + luSubGap) * u.sM

	// login field
	fieldW := pw - 12*u.sM
	focusUser := u.phase == phaseUser && !u.focusPw
	u.hitUser = rect{cx, cy, fieldW, luField * u.sM}
	u.renderField(f, cx, cy, fieldW, txtLabelLogin, string(u.username), focusUser, false, now)
	cy += luField * u.sM

	// second slot: the local password field until pam asks for
	// something else ghosted while busy
	u.hitPrompt = rect{cx, cy, fieldW, luField * u.sM}
	switch {
	case u.phase == phasePrompt || (u.phase == phaseBusy && u.prompt != nil):
		label := strings.ToUpper(strings.TrimRight(strings.TrimSpace(u.prompt.GetMessage()), ":"))
		if label == "" {
			label = txtLabelPassword
		}
		secret := u.prompt.GetType() == pb.PromptType_PROMPT_TYPE_SECRET
		u.renderField(f, cx, cy, fieldW, label, string(u.input), u.phase == phasePrompt, secret, now)
	case u.phase == phaseUser:
		u.renderField(f, cx, cy, fieldW, txtLabelPassword, string(u.input), u.focusPw, true, now)
	default:
		u.renderGhostField(f, cx, cy, fieldW, txtLabelPassword)
	}
	cy += luField * u.sM

	// session selector
	cy += luSessGap * u.sM
	u.hitSess = rect{cx, cy - u.sM, fieldW, (luSess + 2) * u.sM}
	name := strings.ToUpper(u.session().GetName())
	sess := txtLabelSession
	f.DrawText(cx, cy, sess, u.sS, colDim)
	arrowL, arrowR := " ", " "
	if len(u.sessions) > 1 {
		arrowL, arrowR = "<", ">"
	}
	line := fmt.Sprintf("%s %s %s", arrowL, name, arrowR)
	f.DrawText(cx+TextWidth(sess+"  ", u.sS), cy, line, u.sS, colAccent)
	badge := sessionBadge(u.session().GetType())
	f.DrawText(px+pw-6*u.sM-TextWidth(badge, u.sS), cy, badge, u.sS, colEmber)
	cy += (luSess + luMsgGap) * u.sM

	// pam info messages
	for i := max(0, len(u.msgs)-2); i < len(u.msgs); i++ {
		msg := u.msgs[i]
		maxChars := (pw - 12*u.sM) / (glyphW * u.sS)
		if len(msg) > maxChars {
			msg = msg[:maxChars]
		}
		f.DrawText(cx, cy, msg, u.sS, colAmber)
		cy += luMsg * u.sM
	}

	u.renderStatus(f, px, py, pw, ph, now)
	u.renderHints(f, px, py, pw, ph)
}

// renderField draws label + boxed input returns next y
func (u *UI) renderField(f *Frame, x, y, w int, label, value string, focused, secret bool, now float64) int {
	f.DrawText(x, y, label, u.sS, colDim)
	y += 8*u.sS + u.sM

	bh := 8*u.sM + 4*u.sM
	f.FillRect(x, y, w, bh, colField)
	border := dim(colAccent, 90)
	if focused {
		pulse := uint32(170 + 80*math.Sin(now*4))
		border = dim(colAccent, pulse)
	}
	f.Border(x, y, w, bh, border)

	tx, ty := x+2*u.sM, y+2*u.sM
	if secret {
		// chunky blocks instead of glyphs
		bs := 5 * u.sM
		for i := 0; i < len(value) && tx+i*(bs+u.sM)+bs < x+w-2*u.sM; i++ {
			f.FillRect(tx+i*(bs+u.sM), ty+u.sM, bs, bs, colAccent)
		}
		tx += len(value) * (bs + u.sM)
	} else {
		// clip from the left so the tail stays visible
		maxChars := (w - 6*u.sM) / (glyphW * u.sM)
		if len(value) > maxChars && maxChars > 0 {
			value = value[len(value)-maxChars:]
		}
		f.DrawText(tx, ty, value, u.sM, colText)
		tx += TextWidth(value, u.sM)
	}
	if focused && math.Mod(now, 1) < 0.55 {
		f.FillRect(tx+u.sM/2, ty, u.sM*2, 8*u.sM, colAccent)
	}
	return y + bh + 3*u.sM
}

// the slot pam will fill kept dim so the panel never looks half empty
func (u *UI) renderGhostField(f *Frame, x, y, w int, label string) {
	f.DrawText(x, y, label, u.sS, dim(colDim, 130))
	y += 8*u.sS + u.sM
	bh := 12 * u.sM
	f.BlendRect(x, y, w, bh, colField, 128)
	f.Border(x, y, w, bh, dim(colAccent, 40))
}

func (u *UI) renderStatus(f *Frame, px, py, pw, ph int, now float64) {
	y := py + ph - (luPadBot+luStatus)*u.sM

	switch {
	case u.phase == phaseBusy:
		dots := strings.Repeat(".", 1+int(now*3)%3)
		msg := u.busyMsg + dots
		f.DrawText(px+(pw-TextWidth(u.busyMsg+"...", u.sS))/2, y, msg, u.sS, colAccent)

	case u.errMsg != "" && now-u.errAt < errorDur:
		w := TextWidth(u.errMsg, u.sS)
		f.BlendRect(px+1, y-2*u.sM, pw-2, 8*u.sS+4*u.sM, colError, 70)
		f.DrawText(px+(pw-w)/2, y, u.errMsg, u.sS, colError)

	case u.powerArm != pb.PowerAction_POWER_ACTION_UNSPECIFIED && now-u.armAt < armDur:
		msg := txtConfirmPower + powerKeyName(u.powerArm)
		f.DrawText(px+(pw-TextWidth(msg, u.sS))/2, y, msg, u.sS, colAmber)
	}
}

// hints live under the panel in smaller type
func (u *UI) renderHints(f *Frame, px, py, pw, ph int) {
	hints := []string{txtHintLogin}
	if len(u.sessions) > 1 {
		hints = append(hints, txtHintSession)
	}
	if u.caps.GetCanPoweroff() {
		hints = append(hints, txtHintPoweroff)
	}
	if u.caps.GetCanReboot() {
		hints = append(hints, txtHintReboot)
	}
	if u.caps.GetCanSuspend() {
		hints = append(hints, txtHintSuspend)
	}
	line := strings.Join(hints, "   ")
	scale := max(1, u.sS-1)
	f.DrawTextShadow((u.w-TextWidth(line, scale))/2, py+ph+4*u.sM, line, scale, colDim, colShadow)
}

func (u *UI) corners(f *Frame, x, y, w, h int) {
	l, t := 8*u.sM, u.sM
	c := colAccent
	// top-left
	f.FillRect(x-t, y-t, l, t, c)
	f.FillRect(x-t, y-t, t, l, c)
	// top-right
	f.FillRect(x+w+t-l, y-t, l, t, c)
	f.FillRect(x+w, y-t, t, l, c)
	// bottom-left
	f.FillRect(x-t, y+h, l, t, c)
	f.FillRect(x-t, y+h+t-l, t, l, c)
	// bottom-right
	f.FillRect(x+w+t-l, y+h, l, t, c)
	f.FillRect(x+w, y+h+t-l, t, l, c)
}

func sessionBadge(t pb.SessionType) string {
	switch t {
	case pb.SessionType_SESSION_TYPE_WAYLAND:
		return txtBadgeWayland
	case pb.SessionType_SESSION_TYPE_X11:
		return txtBadgeX11
	default:
		return txtBadgeTTY
	}
}

// boot reveal and warp fade sit above the crt pass
func (u *UI) renderOverlays(f *Frame, now float64) {
	if now < bootDur {
		k := now / bootDur
		k = k * k * (3 - 2*k) // smoothstep
		slit := int(float64(u.h) / 2 * k)
		mid := u.h / 2
		f.FillRect(0, 0, u.w, mid-slit, colShadow)
		f.FillRect(0, mid+slit, u.w, u.h-(mid+slit), colShadow)
		if slit < mid {
			f.FillRect(0, mid-slit-u.sS, u.w, u.sS, dim(colAccent, uint32(256*(1-k))))
			f.FillRect(0, mid+slit, u.w, u.sS, dim(colAccent, uint32(256*(1-k))))
		}
	}

	if u.phase == phaseWarp {
		k := (now - u.warpAt) / warpDur
		if k > 1 {
			k = 1
		}
		if k < 0.2 {
			// white flash then fade to black
			f.BlendRect(0, 0, u.w, u.h, colText, uint32(k/0.2*90))
		} else {
			a := uint32((k - 0.2) / 0.8 * 256)
			f.BlendRect(0, 0, u.w, u.h, 0, a)
		}
	}
}
