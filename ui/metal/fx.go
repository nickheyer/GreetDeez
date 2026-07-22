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

const (
	blobCount = 5
	blobLutN  = 512
)

// blob is one lava body drifting through the plasma grid.
type blob struct {
	x, y   float64
	r      float64 // base radius in grid px
	vx, vy float64
	phase  float64 // desyncs the buoyancy wobble per blob
}

type Plasma struct {
	w, h int // low-res grid
	pal  [256]uint32
	sin  [sinSteps]int32 // -127..127
	dist []uint16        // radial table scaled for sin lookups
	idx  []uint8         // palette index buffer

	// lava layer: blobs write a half-res heat field that glows ember
	// over the palette, and the pointer stirs it around
	heat    []uint8
	glow    [256]uint32 // heat -> additive ember color
	blobs   [blobCount]blob
	lut     [blobLutN + 1]uint8 // normalized d^2 -> heat falloff
	t       float64
	px, py  float64 // pointer in grid coords
	energy  float64 // pointer heat, spikes with motion and clicks, decays
	tracked bool    // pointer has been seen at least once
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
	p.heat = make([]uint8, w*h)
	p.buildPalette()
	p.buildLava()
	return p
}

func (p *Plasma) buildLava() {
	// smooth metaball falloff: (1-q)^2 over normalized squared distance
	for i := 0; i <= blobLutN; i++ {
		q := float64(i) / blobLutN
		p.lut[i] = uint8(150 * (1 - q) * (1 - q))
	}
	// ember ramp: deep red-orange up into near-white cores
	for h := 0; h < 256; h++ {
		r := min(uint32(h)*2, 255)
		g := uint32(h) * 3 / 5
		b := uint32(h) / 5
		if h > 190 { // hot core bleaches toward white
			k := uint32(h-190) * 2
			g = min(g+k, 255)
			b = min(b+k, 255)
		}
		p.glow[h] = rgb(r, g, b)
	}
	rng := rand.New(rand.NewSource(0x1a7a))
	for i := range p.blobs {
		p.blobs[i] = blob{
			x:     (0.15 + 0.7*rng.Float64()) * float64(p.w),
			y:     (0.2 + 0.6*rng.Float64()) * float64(p.h),
			r:     float64(p.h) * (0.06 + 0.05*rng.Float64()),
			phase: rng.Float64() * 2 * math.Pi,
		}
	}
}

// SetPointer feeds pointer position (grid coords) and speed (grid px moved).
func (p *Plasma) SetPointer(x, y, speed float64) {
	p.px, p.py = x, y
	p.tracked = true
	p.energy = math.Min(1, p.energy+speed*0.012)
}

// Pulse is a click splash: heat spike plus a shove on every blob.
func (p *Plasma) Pulse() {
	if !p.tracked {
		return
	}
	p.energy = 1
	for i := range p.blobs {
		b := &p.blobs[i]
		dx, dy := b.x-p.px, b.y-p.py
		d := math.Hypot(dx, dy) + 8
		kick := 900 / d
		b.vx += dx / d * kick
		b.vy += dy / d * kick
	}
}

// Update advances the lava: buoyancy wander, pointer attraction, damping.
func (p *Plasma) Update(dt float64) {
	p.t += dt
	p.energy *= math.Exp(-dt * 1.6)
	w, h := float64(p.w), float64(p.h)
	for i := range p.blobs {
		b := &p.blobs[i]
		// slow convection cells: rise, stall, sink
		b.vx += math.Sin(p.t*0.11+b.phase*2.3) * 3.2 * dt
		b.vy += math.Sin(p.t*0.17+b.phase) * 4.6 * dt
		if p.tracked && p.energy > 0.02 {
			// lazy drift toward the pointer while it is moving
			dx, dy := p.px-b.x, p.py-b.y
			d := math.Hypot(dx, dy) + 24
			pull := p.energy * 140 * dt / d
			b.vx += dx * pull
			b.vy += dy * pull
		}
		damp := math.Exp(-dt * 0.7)
		b.vx *= damp
		b.vy *= damp
		b.x += b.vx * dt
		b.y += b.vy * dt
		// soft walls keep the lava on screen
		if b.x < b.r*0.5 {
			b.x, b.vx = b.r*0.5, math.Abs(b.vx)*0.6
		}
		if b.x > w-b.r*0.5 {
			b.x, b.vx = w-b.r*0.5, -math.Abs(b.vx)*0.6
		}
		if b.y < b.r*0.5 {
			b.y, b.vy = b.r*0.5, math.Abs(b.vy)*0.6
		}
		if b.y > h-b.r*0.5 {
			b.y, b.vy = h-b.r*0.5, -math.Abs(b.vy)*0.6
		}
	}
}

// stampHeat rebuilds the half-res heat field from blobs and pointer.
func (p *Plasma) stampHeat() {
	for i := range p.heat {
		p.heat[i] = 0
	}
	type stamp struct {
		x, y, r float64
		gain    uint32 // 0..256 scales the falloff
	}
	stamps := make([]stamp, 0, blobCount+1)
	for i := range p.blobs {
		b := &p.blobs[i]
		re := b.r * (1 + 0.15*math.Sin(p.t*0.4+b.phase))
		stamps = append(stamps, stamp{b.x, b.y, re, 256})
	}
	if p.tracked && p.energy > 0.02 {
		// the pointer is its own small hot blob so wiggling the mouse
		// literally stirs light into the lava
		r := 6 + p.energy*float64(p.h)*0.05
		stamps = append(stamps, stamp{p.px, p.py, r, uint32(90 + 166*p.energy)})
	}
	for _, s := range stamps {
		r := int(s.r)
		if r < 2 {
			continue
		}
		// fixed point: lutIndex = d2 * blobLutN / r^2
		invR := (blobLutN << 12) / int(s.r*s.r)
		bx, by := int(s.x), int(s.y)
		y0, y1 := max(by-r, 0), min(by+r, p.h-1)
		x0, x1 := max(bx-r, 0), min(bx+r, p.w-1)
		for y := y0; y <= y1; y++ {
			dy := y - by
			dy2 := dy * dy
			row := p.heat[y*p.w : (y+1)*p.w]
			dx := x0 - bx
			d2 := dx*dx + dy2
			step := 2*dx + 1
			for x := x0; x <= x1; x++ {
				li := (d2 * invR) >> 12
				if li <= blobLutN {
					v := uint32(row[x]) + uint32(p.lut[li])*s.gain>>8
					if v > 255 {
						v = 255
					}
					row[x] = uint8(v)
				}
				d2 += step
				step += 2
			}
		}
	}
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

	p.stampHeat()

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

	// expand 2x through palette with the lava glow layered on top
	parallelRows(f.H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			sy := min(y/2, p.h-1)
			src := p.idx[sy*p.w : (sy+1)*p.w]
			hrow := p.heat[sy*p.w : (sy+1)*p.w]
			dst := f.Pix[y*f.W : (y+1)*f.W]
			for x := range dst {
				sx := x / 2
				if sx >= p.w {
					sx = p.w - 1
				}
				c := p.pal[src[sx]]
				if h := hrow[sx]; h != 0 {
					c = addColor(c, p.glow[h])
				}
				dst[x] = c
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
