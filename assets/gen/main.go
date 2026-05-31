// Command gen renders the tray/menubar status icons (green, yellow, red, grey)
// as both PNG (used on the macOS menubar) and ICO (used in the Windows tray),
// writing them into the assets directory.
//
// The icons are generated from code — not hand-authored binary blobs — so they
// are fully reproducible in this trust repo. Regenerate with:
//
//	go run ./assets/gen
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
)

// statusColors maps each state name to its dot color (opaque).
var statusColors = map[string]color.RGBA{
	"green":  {R: 46, G: 160, B: 67, A: 255},   // Hub healthy + protected
	"yellow": {R: 227, G: 179, B: 65, A: 255},  // Hub running, health degraded
	"red":    {R: 248, G: 81, B: 73, A: 255},   // Hub down / unreachable
	"grey":   {R: 139, G: 148, B: 158, A: 255}, // app starting up
}

func main() {
	outDir := flag.String("out", "assets", "directory to write icons into")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("create out dir: %v", err)
	}

	for name, col := range statusColors {
		png16 := renderCircle(16, col)
		png32 := renderCircle(32, col)

		// PNG (macOS menubar) — 32px scales cleanly to the ~22pt bar.
		if err := writePNG(filepath.Join(*outDir, "icon_"+name+".png"), png32); err != nil {
			log.Fatalf("write %s png: %v", name, err)
		}

		// ICO (Windows tray) — bundle 16px and 32px so Windows picks the
		// crispest size for the current DPI.
		ico, err := encodeICO([]*image.RGBA{png16, png32})
		if err != nil {
			log.Fatalf("encode %s ico: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(*outDir, "icon_"+name+".ico"), ico, 0o644); err != nil {
			log.Fatalf("write %s ico: %v", name, err)
		}
		log.Printf("wrote icon_%s.png + icon_%s.ico", name, name)
	}
}

// renderCircle draws an anti-aliased filled circle of the given color on a
// transparent square of side `size`. It supersamples 4x and box-downscales so
// the edge alpha is smooth while the RGB stays the true color (no dark fringe).
func renderCircle(size int, col color.RGBA) *image.RGBA {
	const ss = 4
	big := size * ss
	src := image.NewRGBA(image.Rect(0, 0, big, big))

	cx, cy := float64(big)/2, float64(big)/2
	r := float64(big)/2 - float64(ss) // tiny margin so the dot isn't clipped
	for y := 0; y < big; y++ {
		for x := 0; x < big; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			a := uint8(0)
			if dx*dx+dy*dy <= r*r {
				a = 255
			}
			// Keep RGB constant everywhere; only alpha distinguishes
			// inside/outside, which yields clean anti-aliased edges.
			src.SetRGBA(x, y, color.RGBA{R: col.R, G: col.G, B: col.B, A: a})
		}
	}

	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	const n = ss * ss
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var rr, gg, bb, aa int
			for j := 0; j < ss; j++ {
				for i := 0; i < ss; i++ {
					c := src.RGBAAt(x*ss+i, y*ss+j)
					rr += int(c.R)
					gg += int(c.G)
					bb += int(c.B)
					aa += int(c.A)
				}
			}
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(rr / n),
				G: uint8(gg / n),
				B: uint8(bb / n),
				A: uint8(aa / n),
			})
		}
	}
	return dst
}

func writePNG(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// encodeICO writes a Windows .ico containing each image as a PNG entry
// (PNG-compressed icons are supported on Windows Vista and later).
func encodeICO(images []*image.RGBA) ([]byte, error) {
	pngs := make([][]byte, len(images))
	for i, img := range images {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
		pngs[i] = buf.Bytes()
	}

	var out bytes.Buffer
	// ICONDIR header.
	_ = binary.Write(&out, binary.LittleEndian, uint16(0)) // reserved
	_ = binary.Write(&out, binary.LittleEndian, uint16(1)) // type: icon
	_ = binary.Write(&out, binary.LittleEndian, uint16(len(pngs)))

	// ICONDIRENTRY for each image; data follows all entries.
	offset := 6 + 16*len(pngs)
	for i, img := range images {
		b := img.Bounds()
		out.WriteByte(dimByte(b.Dx()))
		out.WriteByte(dimByte(b.Dy()))
		out.WriteByte(0)                                        // palette color count (0 = none)
		out.WriteByte(0)                                        // reserved
		_ = binary.Write(&out, binary.LittleEndian, uint16(1))  // color planes
		_ = binary.Write(&out, binary.LittleEndian, uint16(32)) // bits per pixel
		_ = binary.Write(&out, binary.LittleEndian, uint32(len(pngs[i])))
		_ = binary.Write(&out, binary.LittleEndian, uint32(offset))
		offset += len(pngs[i])
	}
	for _, p := range pngs {
		out.Write(p)
	}
	return out.Bytes(), nil
}

// dimByte encodes an icon dimension; the ICO format stores 256 as 0.
func dimByte(v int) byte {
	if v >= 256 {
		return 0
	}
	return byte(v)
}
