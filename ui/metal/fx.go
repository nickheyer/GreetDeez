package metal

import (
	"math"
	"math/rand"
)

// ── Plasma ──────────────────────────────────────────────────────
// classic sin-sum plasma computed at half res with int tables then
// expanded 2x through a cyclic palette

const (
	sinSteps = 1024
	sinMask  = sinSteps - 1
)

type Plasma struct {
	w, h int // low-res grid
	pal  [256]uint32
	sin  [sinSteps]int32 // -127..127
	dist []uint16        // radial table scaled for sin lookups
	idx  []uint8         // palette index buffer
}

func NewPlasma(w, h int) *Plasma {
	p := &Plasma{w: w, h: h}
	for i := range p.sin {
		p.sin[i] = int32(math.Round(math.Sin(2*math.Pi*float64(i)/sinSteps) * 127))
	}
	p.dist = make([]uint16, w*h)
	cx, cy := float64(w)/2, float64(h)/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			d := math.Hypot(float64(x)-cx, float64(y)-cy)
			p.dist[y*w+x] = uint16(d * 6)
		}
	}
	p.idx = make([]uint8, w*h)
	p.buildPalette()
	return p
}

// cosine palette dark steel blues into teal with ember highlights
// kept dim so the ui reads on top of it
func (p *Plasma) buildPalette() {
	for i := 0; i < 256; i++ {
		t := float64(i) / 256
		r := palChan(t, 0.16, 0.14, 1.0, 0.10)
		g := palChan(t, 0.16, 0.13, 1.0, 0.32)
		b := palChan(t, 0.22, 0.18, 1.0, 0.55)
		// ember vein a narrow warm band
		e := math.Exp(-math.Pow((t-0.82)*9, 2))
		r += e * 0.30
		g += e * 0.12
		p.pal[i] = rgb(palClamp(r), palClamp(g), palClamp(b))
	}
}

func palChan(t, a, b, c, d float64) float64 {
	return a + b*math.Cos(2*math.Pi*(c*t+d))
}

func palClamp(v float64) uint32 {
	v = math.Max(0, math.Min(1, v))
	return uint32(v * 255)
}

// Render fills the whole frame expanding each plasma cell 2x2.
func (p *Plasma) Render(f *Frame, t float64) {
	t1 := int32(t*90) & sinMask
	t2 := int32(t*63) & sinMask
	t3 := int32(t*44) & sinMask
	t4 := int32(t*31) & sinMask

	parallelRows(p.h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			ys := p.sin[(int32(y*7)+t2)&sinMask]
			row := p.idx[y*p.w : (y+1)*p.w]
			drow := p.dist[y*p.w : (y+1)*p.w]
			for x := range row {
				v := p.sin[(int32(x*5)+t1)&sinMask] +
					ys +
					p.sin[(int32(x*3+y*4)+t3)&sinMask] +
					p.sin[(int32(drow[x])+t4)&sinMask]
				row[x] = uint8((v + 508) >> 2)
			}
		}
	})

	// expand 2x through palette
	parallelRows(f.H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			sy := min(y/2, p.h-1)
			src := p.idx[sy*p.w : (sy+1)*p.w]
			dst := f.Pix[y*f.W : (y+1)*f.W]
			for x := range dst {
				sx := x / 2
				if sx >= p.w {
					sx = p.w - 1
				}
				dst[x] = p.pal[src[sx]]
			}
		}
	})
}

// ── Starfield ───────────────────────────────────────────────────

type star struct {
	x, y, z float32 // z depth 1 (far) .. 0 (near)
}

type Starfield struct {
	stars []star
	w, h  int
	warp  float32 // 0 normal, ramps up during warp-out
	rng   *rand.Rand
}

func NewStarfield(w, h, n int) *Starfield {
	s := &Starfield{w: w, h: h, rng: rand.New(rand.NewSource(0xdee5))}
	s.stars = make([]star, n)
	for i := range s.stars {
		s.stars[i] = s.spawn()
	}
	return s
}

func (s *Starfield) spawn() star {
	return star{
		x: s.rng.Float32()*2 - 1,
		y: s.rng.Float32()*2 - 1,
		z: s.rng.Float32()*0.9 + 0.1,
	}
}

// SetWarp ramps star speed for the login warp-out.
func (s *Starfield) SetWarp(w float32) { s.warp = w }

func (s *Starfield) Update(dt float32) {
	speed := 0.045 + s.warp*3.5
	for i := range s.stars {
		s.stars[i].z -= speed * dt
		if s.stars[i].z <= 0.02 {
			s.stars[i] = s.spawn()
			s.stars[i].z = 1
		}
	}
}

func (s *Starfield) Render(f *Frame) {
	cx, cy := float32(s.w)/2, float32(s.h)/2
	scale := float32(s.h) * 0.5
	for i := range s.stars {
		st := &s.stars[i]
		px := int(cx + st.x/st.z*scale)
		py := int(cy + st.y/st.z*scale)
		if px < 0 || py < 0 || px >= s.w || py >= s.h {
			continue
		}
		// nearer stars are brighter and blue-white
		b := uint32((1 - st.z) * 230)
		c := rgb(b*3/4, b*7/8, b)
		if s.warp > 0 {
			// streak toward the viewer
			z2 := st.z + 0.05 + s.warp*0.09
			qx := int(cx + st.x/z2*scale)
			qy := int(cy + st.y/z2*scale)
			drawLineAdd(f, qx, qy, px, py, c)
		} else {
			f.Add(px, py, c)
			if st.z < 0.3 {
				f.Add(px+1, py, dim(c, 128))
				f.Add(px, py+1, dim(c, 128))
			}
		}
	}
}

// additive bresenham
func drawLineAdd(f *Frame, x0, y0, x1, y1 int, c uint32) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := sign(x1-x0), sign(y1-y0)
	e := dx + dy
	for i := 0; i < 4096; i++ {
		f.Add(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			return
		}
		if 2*e >= dy {
			e += dy
			x0 += sx
		}
		if 2*e <= dx {
			e += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sign(v int) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}
	return 0
}

// ── Sine scroller ───────────────────────────────────────────────

type Scroller struct {
	text  string
	runes []rune
	x     float64
	scale int
}

func NewScroller(text string, scale int) *Scroller {
	return &Scroller{text: text, runes: []rune(text), scale: scale}
}

func (s *Scroller) Update(dt float64, w int) {
	s.x -= dt * 140
	if s.x < -float64(len(s.runes)*glyphW*s.scale) {
		s.x = float64(w)
	}
}

func (s *Scroller) Render(f *Frame, baseY int, t float64) {
	adv := glyphW * s.scale
	for i, ch := range s.runes {
		x := int(s.x) + i*adv
		if x < -adv || x > f.W {
			continue
		}
		phase := float64(x)*0.013 + t*2.6
		y := baseY + int(math.Sin(phase)*float64(6*s.scale))
		hue := (math.Sin(phase*0.5) + 1) / 2
		col := lerpColor(rgb(0x2e, 0xd4, 0xc3), rgb(0xff, 0x8a, 0x3d), uint32(hue*256))
		f.drawGlyph(x, y, glyph(ch), s.scale, col, dim(col, 110))
	}
}

// ── CRT pass ────────────────────────────────────────────────────

// scanlines plus corner vignette applied in place
type CRT struct {
	vignRow []uint32 // 0..256 per row
	vignCol []uint32
}

func NewCRT(w, h int) *CRT {
	c := &CRT{vignRow: make([]uint32, h), vignCol: make([]uint32, w)}
	for y := range c.vignRow {
		c.vignRow[y] = vignFactor(float64(y)/float64(h-1), 0.30)
	}
	for x := range c.vignCol {
		c.vignCol[x] = vignFactor(float64(x)/float64(w-1), 0.22)
	}
	return c
}

func vignFactor(t, strength float64) uint32 {
	d := math.Abs(t-0.5) * 2 // 0 center 1 edge
	f := 1 - strength*d*d
	return uint32(f * 256)
}

func (c *CRT) Apply(f *Frame) {
	parallelRows(f.H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			rf := c.vignRow[y]
			if y%3 == 2 {
				rf = rf * 200 >> 8 // scanline
			}
			row := f.Pix[y*f.W : (y+1)*f.W]
			for x := range row {
				fac := rf * c.vignCol[x] >> 8
				if fac >= 256 {
					continue
				}
				row[x] = dim(row[x], fac)
			}
		}
	})
}
