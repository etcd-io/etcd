package structlayout

import "fmt"

type Field struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Start     int64  `json:"start"`
	End       int64  `json:"end"`
	Size      int64  `json:"size"`
	Align     int64  `json:"align"`
	IsPadding bool   `json:"is_padding"`
}

func (f Field) String() string {
	if f.IsPadding {
		return fmt.Sprintf("%s: %d-%d (size %d, align %d)",
			"padding", f.Start, f.End, f.Size, f.Align)
	}
	return fmt.Sprintf("%s %s: %d-%d (size %d, align %d)",
		f.Name, f.Type, f.Start, f.End, f.Size, f.Align)
}
