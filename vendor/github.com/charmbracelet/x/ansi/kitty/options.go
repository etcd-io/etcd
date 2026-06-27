package kitty

import (
	"encoding"
	"fmt"
	"strconv"
	"strings"
)

var (
	_ encoding.TextMarshaler   = Options{}
	_ encoding.TextUnmarshaler = &Options{}
)

type Stringish interface{ string | []byte }

// Options represents a Kitty Graphics Protocol options.
type Options struct {
	// Common options.

	// Action (a=t) is the action to be performed on the image. Can be one of
	// [Transmit], [TransmitDisplay], [Query], [Put], [Delete], [Frame],
	// [Animate], [Compose].
	Action byte

	// Quite mode (q=0) is the quiet mode. Can be either zero, one, or two
	// where zero is the default, 1 suppresses OK responses, and 2 suppresses
	// both OK and error responses.
	Quite byte

	// Transmission options.

	// ID (i=) is the image ID. The ID is a unique identifier for the image.
	// Must be a positive integer up to [math.MaxUint32].
	ID int

	// PlacementID (p=) is the placement ID. The placement ID is a unique
	// identifier for the placement of the image. Must be a positive integer up
	// to [math.MaxUint32].
	PlacementID int

	// Number (I=0) is the number of images to be transmitted.
	Number int

	// Format (f=32) is the image format. One of [RGBA], [RGB], [PNG].
	Format int

	// ImageWidth (s=0) is the transmitted image width.
	ImageWidth int

	// ImageHeight (v=0) is the transmitted image height.
	ImageHeight int

	// Compression (o=) is the image compression type. Can be [Zlib] or zero.
	Compression byte

	// Transmission (t=d) is the image transmission type. Can be [Direct], [File],
	// [TempFile], or[SharedMemory].
	Transmission byte

	// File is the file path to be used when the transmission type is [File].
	// If [Options.Transmission] is omitted i.e. zero and this is non-empty,
	// the transmission type is set to [File].
	File string

	// Size (S=0) is the size to be read from the transmission medium.
	Size int

	// Offset (O=0) is the offset byte to start reading from the transmission
	// medium.
	Offset int

	// Chunk (m=) whether the image is transmitted in chunks. Can be either
	// zero or one. When true, the image is transmitted in chunks. Each chunk
	// must be a multiple of 4, and up to [MaxChunkSize] bytes. Each chunk must
	// have the m=1 option except for the last chunk which must have m=0.
	Chunk bool

	// ChunkFormatter is the function used to format each chunk when
	// [Options.Chunk] is true. If nil, the chunks are sent as is.
	ChunkFormatter func(chunk string) string

	// Display options.

	// X (x=0) is the pixel X coordinate of the image to start displaying.
	X int

	// Y (y=0) is the pixel Y coordinate of the image to start displaying.
	Y int

	// Z (z=0) is the Z coordinate of the image to display.
	Z int

	// Width (w=0) is the width of the image to display.
	Width int

	// Height (h=0) is the height of the image to display.
	Height int

	// OffsetX (X=0) is the OffsetX coordinate of the cursor cell to start
	// displaying the image. OffsetX=0 is the leftmost cell. This must be
	// smaller than the terminal cell width.
	OffsetX int

	// OffsetY (Y=0) is the OffsetY coordinate of the cursor cell to start
	// displaying the image. OffsetY=0 is the topmost cell. This must be
	// smaller than the terminal cell height.
	OffsetY int

	// Columns (c=0) is the number of columns to display the image. The image
	// will be scaled to fit the number of columns.
	Columns int

	// Rows (r=0) is the number of rows to display the image. The image will be
	// scaled to fit the number of rows.
	Rows int

	// VirtualPlacement (U=0) whether to use virtual placement. This is used
	// with Unicode [Placeholder] to display images.
	VirtualPlacement bool

	// DoNotMoveCursor (C=0) whether to move the cursor after displaying the
	// image.
	DoNotMoveCursor bool

	// ParentID (P=0) is the parent image ID. The parent ID is the ID of the
	// image that is the parent of the current image. This is used with Unicode
	// [Placeholder] to display images relative to the parent image.
	ParentID int

	// ParentPlacementID (Q=0) is the parent placement ID. The parent placement
	// ID is the ID of the placement of the parent image. This is used with
	// Unicode [Placeholder] to display images relative to the parent image.
	ParentPlacementID int

	// Delete options.

	// Delete (d=a) is the delete action. Can be one of [DeleteAll],
	// [DeleteID], [DeleteNumber], [DeleteCursor], [DeleteFrames],
	// [DeleteCell], [DeleteCellZ], [DeleteRange], [DeleteColumn], [DeleteRow],
	// [DeleteZ].
	Delete byte

	// DeleteResources indicates whether to delete the resources associated
	// with the image.
	DeleteResources bool
}

// Options returns the options as a slice of a key-value pairs.
func (o *Options) Options() (opts []string) {
	opts = []string{}
	if o.Format == 0 {
		o.Format = RGBA
	}

	if o.Action == 0 {
		o.Action = Transmit
	}

	if o.Delete == 0 {
		o.Delete = DeleteAll
	}

	if o.Transmission == 0 {
		if len(o.File) > 0 {
			o.Transmission = File
		} else {
			o.Transmission = Direct
		}
	}

	if o.Format != RGBA {
		opts = append(opts, fmt.Sprintf("f=%d", o.Format))
	}

	if o.Quite > 0 {
		opts = append(opts, fmt.Sprintf("q=%d", o.Quite))
	}

	if o.ID > 0 {
		opts = append(opts, fmt.Sprintf("i=%d", o.ID))
	}

	if o.PlacementID > 0 {
		opts = append(opts, fmt.Sprintf("p=%d", o.PlacementID))
	}

	if o.Number > 0 {
		opts = append(opts, fmt.Sprintf("I=%d", o.Number))
	}

	if o.ImageWidth > 0 {
		opts = append(opts, fmt.Sprintf("s=%d", o.ImageWidth))
	}

	if o.ImageHeight > 0 {
		opts = append(opts, fmt.Sprintf("v=%d", o.ImageHeight))
	}

	if o.Transmission != Direct {
		opts = append(opts, fmt.Sprintf("t=%c", o.Transmission))
	}

	if o.Size > 0 {
		opts = append(opts, fmt.Sprintf("S=%d", o.Size))
	}

	if o.Offset > 0 {
		opts = append(opts, fmt.Sprintf("O=%d", o.Offset))
	}

	if o.Compression == Zlib {
		opts = append(opts, fmt.Sprintf("o=%c", o.Compression))
	}

	if o.VirtualPlacement {
		opts = append(opts, "U=1")
	}

	if o.DoNotMoveCursor {
		opts = append(opts, "C=1")
	}

	if o.ParentID > 0 {
		opts = append(opts, fmt.Sprintf("P=%d", o.ParentID))
	}

	if o.ParentPlacementID > 0 {
		opts = append(opts, fmt.Sprintf("Q=%d", o.ParentPlacementID))
	}

	if o.X > 0 {
		opts = append(opts, fmt.Sprintf("x=%d", o.X))
	}

	if o.Y > 0 {
		opts = append(opts, fmt.Sprintf("y=%d", o.Y))
	}

	if o.Z > 0 {
		opts = append(opts, fmt.Sprintf("z=%d", o.Z))
	}

	if o.Width > 0 {
		opts = append(opts, fmt.Sprintf("w=%d", o.Width))
	}

	if o.Height > 0 {
		opts = append(opts, fmt.Sprintf("h=%d", o.Height))
	}

	if o.OffsetX > 0 {
		opts = append(opts, fmt.Sprintf("X=%d", o.OffsetX))
	}

	if o.OffsetY > 0 {
		opts = append(opts, fmt.Sprintf("Y=%d", o.OffsetY))
	}

	if o.Columns > 0 {
		opts = append(opts, fmt.Sprintf("c=%d", o.Columns))
	}

	if o.Rows > 0 {
		opts = append(opts, fmt.Sprintf("r=%d", o.Rows))
	}

	if o.Delete != DeleteAll || o.DeleteResources {
		da := o.Delete
		if o.DeleteResources {
			da = da - ' ' // to uppercase
		}

		opts = append(opts, fmt.Sprintf("d=%c", da))
	}

	if o.Action != Transmit {
		opts = append(opts, fmt.Sprintf("a=%c", o.Action))
	}

	return opts // complex function with multiple returns
}

// String returns the string representation of the options.
func (o Options) String() string {
	return strings.Join(o.Options(), ",")
}

// MarshalText returns the string representation of the options.
func (o Options) MarshalText() ([]byte, error) {
	return []byte(o.String()), nil
}

// UnmarshalText parses the options from the given string.
func (o *Options) UnmarshalText(text []byte) error {
	opts := strings.Split(string(text), ",")
	for _, opt := range opts {
		ps := strings.SplitN(opt, "=", 2)
		if len(ps) != 2 || len(ps[1]) == 0 {
			continue
		}

		switch ps[0] {
		case "a":
			o.Action = ps[1][0]
		case "o":
			o.Compression = ps[1][0]
		case "t":
			o.Transmission = ps[1][0]
		case "d":
			d := ps[1][0]
			if d >= 'A' && d <= 'Z' {
				o.DeleteResources = true
				d = d + ' ' // to lowercase
			}
			o.Delete = d
		case "i", "q", "p", "I", "f", "s", "v", "S", "O", "m", "x", "y", "z", "w", "h", "X", "Y", "c", "r", "U", "P", "Q":
			v, err := strconv.Atoi(ps[1])
			if err != nil {
				continue
			}

			switch ps[0] {
			case "i":
				o.ID = v
			case "q":
				o.Quite = byte(v)
			case "p":
				o.PlacementID = v
			case "I":
				o.Number = v
			case "f":
				o.Format = v
			case "s":
				o.ImageWidth = v
			case "v":
				o.ImageHeight = v
			case "S":
				o.Size = v
			case "O":
				o.Offset = v
			case "m":
				o.Chunk = v == 0 || v == 1
			case "x":
				o.X = v
			case "y":
				o.Y = v
			case "z":
				o.Z = v
			case "w":
				o.Width = v
			case "h":
				o.Height = v
			case "X":
				o.OffsetX = v
			case "Y":
				o.OffsetY = v
			case "c":
				o.Columns = v
			case "r":
				o.Rows = v
			case "U":
				o.VirtualPlacement = v == 1
			case "P":
				o.ParentID = v
			case "Q":
				o.ParentPlacementID = v
			}
		}
	}

	return nil
}
