// Package convert turns comic archives (CBZ/EPUB/PDF) into AVIF page images.
package convert

import (
	"bytes"
	"fmt"
	"image"

	// Input decoders. gen2brain/avif also self-registers an AVIF decoder.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/gen2brain/avif"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// EncodeOptions controls AVIF output.
type EncodeOptions struct {
	Quality int // 0-100
	Speed   int // 0 (best) .. 10 (fastest)
	MaxEdge int // downscale longest edge to this (0 = keep original)
}

// encodedPage is one produced page.
type encodedPage struct {
	Index  int
	Data   []byte
	Width  int
	Height int
}

// decodeAndEncode decodes an arbitrary raster image and re-encodes it to AVIF,
// downscaling if it exceeds MaxEdge.
func decodeAndEncode(raw []byte, idx int, opt EncodeOptions) (encodedPage, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return encodedPage{}, fmt.Errorf("decode page %d: %w", idx, err)
	}

	img = downscale(img, opt.MaxEdge)
	b := img.Bounds()

	var buf bytes.Buffer
	if err := avif.Encode(&buf, img, avif.Options{
		Quality:      opt.Quality,
		QualityAlpha: opt.Quality,
		Speed:        opt.Speed,
	}); err != nil {
		return encodedPage{}, fmt.Errorf("avif encode page %d: %w", idx, err)
	}
	return encodedPage{
		Index:  idx,
		Data:   buf.Bytes(),
		Width:  b.Dx(),
		Height: b.Dy(),
	}, nil
}

// downscale returns a resized copy when the longest edge exceeds maxEdge.
func downscale(src image.Image, maxEdge int) image.Image {
	if maxEdge <= 0 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	longest := w
	if h > longest {
		longest = h
	}
	if longest <= maxEdge {
		return src
	}
	scale := float64(maxEdge) / float64(longest)
	nw, nh := int(float64(w)*scale), int(float64(h)*scale)
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}
