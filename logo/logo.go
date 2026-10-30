// Package logo draws the AppCloser icon: a blue rounded square with a white
// power symbol.
package logo

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
)

type Palette struct{ Top, Bottom color.NRGBA }

var (
	Blue = Palette{color.NRGBA{0x4F, 0xA3, 0xFF, 0xFF}, color.NRGBA{0x1D, 0x5F, 0xD6, 0xFF}}
	Gray = Palette{color.NRGBA{0xA8, 0xAE, 0xB8, 0xFF}, color.NRGBA{0x6B, 0x72, 0x7D, 0xFF}}
)

func segDist(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	t := math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/(dx*dx+dy*dy)))
	return math.Hypot(px-ax-t*dx, py-ay-t*dy)
}

// Draw renders the icon at the given pixel size.
func Draw(size int, p Palette) *image.NRGBA {
	const ss = 4
	s := float64(size)
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	margin, radius := s*0.04, s*0.24
	cx, cy := s/2, s*0.535
	ringR, stroke := s*0.235, s*0.085
	gap := 42 * math.Pi / 180 // half opening angle at the top
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var bg, fg int
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px := float64(x) + (float64(sx)+0.5)/ss
					py := float64(y) + (float64(sy)+0.5)/ss
					// rounded square
					qx := math.Max(math.Abs(px-s/2)-(s/2-margin-radius), 0)
					qy := math.Max(math.Abs(py-s/2)-(s/2-margin-radius), 0)
					if math.Hypot(qx, qy) > radius {
						continue
					}
					bg++
					// power ring with round caps
					ang := math.Atan2(px-cx, -(py - cy)) // 0 = straight up
					var d float64
					if math.Abs(ang) >= gap {
						d = math.Abs(math.Hypot(px-cx, py-cy) - ringR)
					} else {
						ex, ey := math.Sin(gap)*ringR, -math.Cos(gap)*ringR
						d = math.Min(math.Hypot(px-cx-ex, py-cy-ey), math.Hypot(px-cx+ex, py-cy-ey))
					}
					// vertical bar
					d = math.Min(d, segDist(px, py, cx, cy-ringR*1.18, cx, cy-ringR*0.12))
					if d <= stroke/2 {
						fg++
					}
				}
			}
			if bg == 0 {
				continue
			}
			t := (float64(x) + float64(y)) / (2 * s)
			lerp := func(a, b uint8) float64 { return float64(a)*(1-t) + float64(b)*t }
			w := float64(fg) / float64(bg)
			mix := func(a, b uint8) uint8 { return uint8(lerp(a, b)*(1-w) + 255*w) }
			img.SetNRGBA(x, y, color.NRGBA{
				mix(p.Top.R, p.Bottom.R), mix(p.Top.G, p.Bottom.G), mix(p.Top.B, p.Bottom.B),
				uint8(255 * float64(bg) / ss / ss),
			})
		}
	}
	return img
}

func PNG(size int, p Palette) []byte {
	var buf bytes.Buffer
	png.Encode(&buf, Draw(size, p))
	return buf.Bytes()
}

// ICO returns a Windows icon file with PNG-compressed entries.
func ICO(p Palette, sizes ...int) []byte {
	var imgs [][]byte
	for _, s := range sizes {
		imgs = append(imgs, PNG(s, p))
	}
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, []uint16{0, 1, uint16(len(sizes))})
	off := 6 + 16*len(sizes)
	for i, s := range sizes {
		dim := uint8(s)
		if s >= 256 {
			dim = 0
		}
		b.Write([]byte{dim, dim, 0, 0})
		binary.Write(&b, binary.LittleEndian, []uint16{1, 32})
		binary.Write(&b, binary.LittleEndian, []uint32{uint32(len(imgs[i])), uint32(off)})
		off += len(imgs[i])
	}
	for _, img := range imgs {
		b.Write(img)
	}
	return b.Bytes()
}
