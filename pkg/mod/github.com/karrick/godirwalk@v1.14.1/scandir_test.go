package godirwalk

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDir(t *testing.T) {
	t.Run("dirents", func(t *testing.T) {
		actual, err := ReadDirents(filepath.Join(testRoot, "d0"), nil)

		ensureError(t, err)

		expected := Dirents{
			&Dirent{
				name:     maxName,
				modeType: os.FileMode(0),
			},
			&Dirent{
				name:     "d1",
				modeType: os.ModeDir,
			},
			&Dirent{
				name:     "f1",
				modeType: os.FileMode(0),
			},
			&Dirent{
				name:     "skips",
				modeType: os.ModeDir,
			},
			&Dirent{
				name:     "symlinks",
				modeType: os.ModeDir,
			},
		}

		ensureDirentsMatch(t, actual, expected)
	})

	t.Run("dirnames", func(t *testing.T) {
		actual, err := ReadDirnames(filepath.Join(testRoot, "d0"), nil)
		ensureError(t, err)
		expected := []string{maxName, "d1", "f1", "skips", "symlinks"}
		ensureStringSlicesMatch(t, actual, expected)
	})
}
