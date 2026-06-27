package tags

import "fmt"

type tagEntry struct {
	Key     string
	Content string
}

// filler is used to collect tags.
// It uses a slice to keep track of the tags in order of appearance.
type filler struct {
	data []*tagEntry

	uniq map[string]*tagEntry
}

func newFiller() *filler {
	return &filler{
		uniq: map[string]*tagEntry{},
	}
}

func (f *filler) Data() []*tagEntry {
	return f.data
}

func (f *filler) Fill(key, value string) error {
	if e, ok := f.uniq[key]; ok {
		e.Content += " " + fmt.Sprintf("%s:%q", key, value)
	} else {
		entry := &tagEntry{
			Key:     key,
			Content: fmt.Sprintf("%s:%q", key, value),
		}

		f.uniq[key] = entry
		f.data = append(f.data, entry)
	}

	return nil
}
