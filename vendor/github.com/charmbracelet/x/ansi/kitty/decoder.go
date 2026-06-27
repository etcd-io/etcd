// Package kitty provides Kitty terminal graphics protocol functionality.
package kitty

import (
	"compress/zlib"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
)

// Decoder is a decoder for the Kitty graphics protocol. It supports decoding
// images in the 24-bit [RGB], 32-bit [RGBA], and [PNG] formats. It can also
// decompress data using zlib.
// The default format is 32-bit [RGBA].
type Decoder struct {
	// Uses zlib decompression.
	Decompress bool

	// Can be one of [RGB], [RGBA], or [PNG].
	Format int

	// Width of the image in pixels. This can be omitted if the image is [PNG]
	// formatted.
	Width int

	// Height of the image in pixels. This can be omitted if the image is [PNG]
	// formatted.
	Height int
}

// Decode decodes the image data from r in the specified format.
func (d *Decoder) Decode(r io.Reader) (image.Image, error) {
	if d.Decompress {
		zr, err := zlib.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("failed to create zlib reader: %w", err)
		}

		defer zr.Close() //nolint:errcheck
		r = zr
	}

	if d.Format == 0 {
		d.Format = RGBA
	}

	switch d.Format {
	case RGBA, RGB:
		return d.decodeRGBA(r, d.Format == RGBA)

	case PNG:
		return png.Decode(r) //nolint:wrapcheck

	default:
		return nil, fmt.Errorf("unsupported format: %d", d.Format)
	}
}

// decodeRGBA decodes the image data in 32-bit RGBA or 24-bit RGB formats.
func (d *Decoder) decodeRGBA(r io.Reader, alpha bool) (image.Image, error) {
	m := image.NewRGBA(image.Rect(0, 0, d.Width, d.Height))

	var buf []byte
	if alpha {
		buf = make([]byte, 4)
	} else {
		buf = make([]byte, 3)
	}

	for y := range d.Height {
		for x := range d.Width {
			if _, err := io.ReadFull(r, buf[:]); err != nil {
				return nil, fmt.Errorf("failed to read pixel data: %w", err)
			}
			if alpha {
				m.SetRGBA(x, y, color.RGBA{buf[0], buf[1], buf[2], buf[3]})
			} else {
				m.SetRGBA(x, y, color.RGBA{buf[0], buf[1], buf[2], 0xff})
			}
		}
	}

	return m, nil
}
