package metal

import (
	"runtime"
	"sync"
)

// Frame is an XRGB8888 pixel buffer (0x00RRGGBB), row-major.
type Frame struct {
	W, H int
	Pix  []uint32
}

func NewFrame(w, h int) *Frame {
	return &Frame{W: w, H: h, Pix: make([]uint32, w*h)}
}

func rgb(r, g, b uint32) uint32 {
	return r<<16 | g<<8 | b
}

func channels(c uint32) (r, g, b uint32) {
	return (c >> 16) & 0xff, (c >> 8) & 0xff, c & 0xff
}

// a is 0..256
func lerpColor(c1, c2 uint32, a uint32) uint32 {
	r1, g1, b1 := channels(c1)
	r2, g2, b2 := channels(c2)
	return rgb(
		(r1*(256-a)+r2*a)>>8,
		(g1*(256-a)+g2*a)>>8,
		(b1*(256-a)+b2*a)>>8,
	)
}

// scales brightness by f (0..256)
func dim(c uint32, f uint32) uint32 {
	r, g, b := channels(c)
	return rgb(r*f>>8, g*f>>8, b*f>>8)
}

// saturating additive blend
func addColor(c1, c2 uint32) uint32 {
	r1, g1, b1 := channels(c1)
	r2, g2, b2 := channels(c2)
	return rgb(min(r1+r2, 255), min(g1+g2, 255), min(b1+b2, 255))
}

func (f *Frame) Clear(c uint32) {
	for i := range f.Pix {
		f.Pix[i] = c
	}
}

func (f *Frame) Set(x, y int, c uint32) {
	if x < 0 || y < 0 || x >= f.W || y >= f.H {
		return
	}
	f.Pix[y*f.W+x] = c
}

func (f *Frame) Add(x, y int, c uint32) {
	if x < 0 || y < 0 || x >= f.W || y >= f.H {
		return
	}
	i := y*f.W + x
	f.Pix[i] = addColor(f.Pix[i], c)
}

// clips rect to frame returns false when fully outside
func (f *Frame) clip(x, y, w, h int) (x0, y0, x1, y1 int, ok bool) {
	x0, y0 = max(x, 0), max(y, 0)
	x1, y1 = min(x+w, f.W), min(y+h, f.H)
	return x0, y0, x1, y1, x0 < x1 && y0 < y1
}

func (f *Frame) FillRect(x, y, w, h int, c uint32) {
	x0, y0, x1, y1, ok := f.clip(x, y, w, h)
	if !ok {
		return
	}
	for yy := y0; yy < y1; yy++ {
		row := f.Pix[yy*f.W+x0 : yy*f.W+x1]
		for i := range row {
			row[i] = c
		}
	}
}

// alpha 0..256 blends c over existing pixels
func (f *Frame) BlendRect(x, y, w, h int, c uint32, alpha uint32) {
	x0, y0, x1, y1, ok := f.clip(x, y, w, h)
	if !ok {
		return
	}
	for yy := y0; yy < y1; yy++ {
		row := f.Pix[yy*f.W+x0 : yy*f.W+x1]
		for i := range row {
			row[i] = lerpColor(row[i], c, alpha)
		}
	}
}

// 1px thick border
func (f *Frame) Border(x, y, w, h int, c uint32) {
	f.FillRect(x, y, w, 1, c)
	f.FillRect(x, y+h-1, w, 1, c)
	f.FillRect(x, y, 1, h, c)
	f.FillRect(x+w-1, y, 1, h, c)
}

// parallelRows splits [0, h) into bands and runs fn on each in parallel.
func parallelRows(h int, fn func(y0, y1 int)) {
	n := runtime.GOMAXPROCS(0)
	if n > h {
		n = h
	}
	if n <= 1 {
		fn(0, h)
		return
	}
	var wg sync.WaitGroup
	band := (h + n - 1) / n
	for y := 0; y < h; y += band {
		end := min(y+band, h)
		wg.Add(1)
		go func(a, b int) {
			defer wg.Done()
			fn(a, b)
		}(y, end)
	}
	wg.Wait()
}
