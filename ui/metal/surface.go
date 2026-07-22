package metal

// Surface is where frames land: real DRM hardware or an in-memory
// buffer for tests and snapshots.
type Surface interface {
	Size() (w, h int)
	// Present copies the frame out and blocks until it is safe to
	// draw the next one (vsync on hardware).
	Present(f *Frame) error
	Close()
}

// memSurface keeps the last presented frame tests and snapshots use it
type memSurface struct {
	w, h int
	last *Frame
}

func newMemSurface(w, h int) *memSurface {
	return &memSurface{w: w, h: h, last: NewFrame(w, h)}
}

func (m *memSurface) Size() (int, int) { return m.w, m.h }

func (m *memSurface) Present(f *Frame) error {
	copy(m.last.Pix, f.Pix)
	return nil
}

func (m *memSurface) Close() {}
