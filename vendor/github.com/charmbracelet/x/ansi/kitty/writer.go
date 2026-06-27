package kitty

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"io"
	"os"

	"github.com/charmbracelet/x/ansi"
)

var (
	// GraphicsTempDir is the directory where temporary files are stored.
	// This is used in [WriteKittyGraphics] along with [os.CreateTemp].
	GraphicsTempDir = ""

	// GraphicsTempPattern is the pattern used to create temporary files.
	// This is used in [WriteKittyGraphics] along with [os.CreateTemp].
	// The Kitty Graphics protocol requires the file path to contain the
	// substring "tty-graphics-protocol".
	GraphicsTempPattern = "tty-graphics-protocol-*"
)

// EncodeGraphics writes an image using the Kitty Graphics protocol with the
// given options to w. It chunks the written data if o.Chunk is true.
//
// You can omit m and use nil when rendering an image from a file. In this
// case, you must provide a file path in o.File and use o.Transmission =
// [File]. You can also use o.Transmission = [TempFile] to write
// the image to a temporary file. In that case, the file path is ignored, and
// the image is written to a temporary file that is automatically deleted by
// the terminal.
//
// See https://sw.kovidgoyal.net/kitty/graphics-protocol/
func EncodeGraphics(w io.Writer, m image.Image, o *Options) error {
	if o == nil {
		o = &Options{}
	}

	if o.Transmission == 0 && len(o.File) != 0 {
		o.Transmission = File
	}

	var data bytes.Buffer // the data to be encoded into base64
	e := &Encoder{
		Compress: o.Compression == Zlib,
		Format:   o.Format,
	}

	switch o.Transmission {
	case Direct:
		if err := e.Encode(&data, m); err != nil {
			return fmt.Errorf("failed to encode direct image: %w", err)
		}

	case SharedMemory:
		//nolint:godox
		// TODO: Implement shared memory
		return fmt.Errorf("shared memory transmission is not yet implemented")

	case File:
		if len(o.File) == 0 {
			return ErrMissingFile
		}

		f, err := os.Open(o.File)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}

		defer f.Close() //nolint:errcheck

		stat, err := f.Stat()
		if err != nil {
			return fmt.Errorf("failed to get file info: %w", err)
		}

		mode := stat.Mode()
		if !mode.IsRegular() {
			return fmt.Errorf("file is not a regular file")
		}

		// Write the file path to the buffer
		if _, err := data.WriteString(f.Name()); err != nil {
			return fmt.Errorf("failed to write file path to buffer: %w", err)
		}

	case TempFile:
		f, err := os.CreateTemp(GraphicsTempDir, GraphicsTempPattern)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		defer f.Close() //nolint:errcheck

		if err := e.Encode(f, m); err != nil {
			return fmt.Errorf("failed to encode image to file: %w", err)
		}

		// Write the file path to the buffer
		if _, err := data.WriteString(f.Name()); err != nil {
			return fmt.Errorf("failed to write file path to buffer: %w", err)
		}
	}

	// Encode image to base64
	var payload bytes.Buffer // the base64 encoded image to be written to w
	b64 := base64.NewEncoder(base64.StdEncoding, &payload)
	if _, err := data.WriteTo(b64); err != nil {
		return fmt.Errorf("failed to write base64 encoded image to payload: %w", err)
	}
	if err := b64.Close(); err != nil {
		return err //nolint:wrapcheck
	}

	// If not chunking, write all at once
	if !o.Chunk {
		_, err := io.WriteString(w, ansi.KittyGraphics(payload.Bytes(), o.Options()...))
		return err //nolint:wrapcheck
	}

	// Write in chunks
	var (
		err error
		n   int
	)
	chunk := make([]byte, MaxChunkSize)
	isFirstChunk := true
	chunkFormatter := o.ChunkFormatter
	if chunkFormatter == nil {
		// Default to no formatting
		chunkFormatter = func(s string) string { return s }
	}

	for {
		// Stop if we read less than the chunk size [MaxChunkSize].
		n, err = io.ReadFull(&payload, chunk)
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read chunk: %w", err)
		}

		opts := buildChunkOptions(o, isFirstChunk, false)
		if _, err := io.WriteString(w,
			chunkFormatter(ansi.KittyGraphics(chunk[:n], opts...))); err != nil {
			return err //nolint:wrapcheck
		}

		isFirstChunk = false
	}

	// Write the last chunk
	opts := buildChunkOptions(o, isFirstChunk, true)
	_, err = io.WriteString(w, chunkFormatter(ansi.KittyGraphics(chunk[:n], opts...)))
	return err //nolint:wrapcheck
}

// buildChunkOptions creates the options slice for a chunk.
func buildChunkOptions(o *Options, isFirstChunk, isLastChunk bool) []string {
	var opts []string
	if isFirstChunk {
		opts = o.Options()
	} else {
		// These options are allowed in subsequent chunks
		if o.Quite > 0 {
			opts = append(opts, fmt.Sprintf("q=%d", o.Quite))
		}
		if o.Action == Frame {
			opts = append(opts, "a=f")
		}
	}

	if !isFirstChunk || !isLastChunk {
		// We don't need to encode the (m=) option when we only have one chunk.
		if isLastChunk {
			opts = append(opts, "m=0")
		} else {
			opts = append(opts, "m=1")
		}
	}
	return opts
}
