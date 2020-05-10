package gopkgs_test

import "testing"
import "github.com/uudashr/gopkgs"

func TestList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skip non-short mode")
	}

	pkgs, err := gopkgs.List(gopkgs.Options{})
	if err != nil {
		t.Fatal("fail getting packages:", err)
	}

	if got := len(pkgs); got == 0 {
		t.Error("got:", got, "want: greater than 0")
	}
}

func BenchmarkList(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := gopkgs.List(gopkgs.Options{}); err != nil {
			b.Fatal("err:", err)
		}
	}
}
