package testdata

import "testing"

func TestBook_Open(t *testing.T) {
	tests := []struct {
		name    string
		b       *Book
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		b := &Book{}
		if err := b.Open(); (err != nil) != tt.wantErr {
			t.Errorf("%q. Book.Open() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func Test_door_Open(t *testing.T) {
	tests := []struct {
		name    string
		d       *door
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		d := &door{}
		if err := d.Open(); (err != nil) != tt.wantErr {
			t.Errorf("%q. door.Open() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func Test_xml_Open(t *testing.T) {
	tests := []struct {
		name    string
		x       *xml
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		x := &xml{}
		if err := x.Open(); (err != nil) != tt.wantErr {
			t.Errorf("%q. xml.Open() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}
