package testdata

import "testing"

func TestSameName(t *testing.T) {
	tests := []struct {
		name    string
		want    int
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SameName()
			if (err != nil) != tt.wantErr {
				t.Errorf("SameName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SameName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sameName(t *testing.T) {
	tests := []struct {
		name    string
		want    int
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sameName()
			if (err != nil) != tt.wantErr {
				t.Errorf("sameName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("sameName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSameTypeName_SameName(t *testing.T) {
	tests := []struct {
		name    string
		t       *SameTypeName
		want    int
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t := &SameTypeName{}
			got, err := t.SameName()
			if (err != nil) != tt.wantErr {
				t.Errorf("SameTypeName.SameName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SameTypeName.SameName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSameTypeName_sameName(t *testing.T) {
	tests := []struct {
		name    string
		t       *SameTypeName
		want    int
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t := &SameTypeName{}
			got, err := t.sameName()
			if (err != nil) != tt.wantErr {
				t.Errorf("SameTypeName.sameName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SameTypeName.sameName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sameTypeName_SameName(t *testing.T) {
	tests := []struct {
		name    string
		t       *sameTypeName
		want    int
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t := &sameTypeName{}
			got, err := t.SameName()
			if (err != nil) != tt.wantErr {
				t.Errorf("sameTypeName.SameName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("sameTypeName.SameName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sameTypeName_sameName(t *testing.T) {
	tests := []struct {
		name    string
		t       *sameTypeName
		want    int
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t := &sameTypeName{}
			got, err := t.sameName()
			if (err != nil) != tt.wantErr {
				t.Errorf("sameTypeName.sameName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("sameTypeName.sameName() = %v, want %v", got, tt.want)
			}
		})
	}
}
