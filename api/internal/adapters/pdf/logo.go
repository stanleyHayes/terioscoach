package pdf

import (
	"bytes"
	"compress/zlib"
	_ "embed"
	"image/png"
)

// Original artwork from apps/web/public/images/brand/terios-logo.svg,
// rasterized, cropped to visible bounds and composited onto white for paper.
//
//go:embed assets/terios-logo.png
var logoPNG []byte

var logoPixels, logoWidth, logoHeight = loadLogo()

func loadLogo() ([]byte, int, int) {
	img, err := png.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		panic("invalid embedded Terios logo: " + err.Error())
	}
	bounds := img.Bounds()
	pixels := make([]byte, 0, bounds.Dx()*bounds.Dy()*3)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			pixels = append(pixels, byte(r>>8), byte(g>>8), byte(b>>8))
		}
	}
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(pixels); err != nil {
		panic(err)
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}
	return compressed.Bytes(), bounds.Dx(), bounds.Dy()
}
