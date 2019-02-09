//+build !go1.8

package service

import (
	"path/filepath"

	"github.com/kardianos/osext"
)

func (c *Config) execPath() (string, error) {
	if len(c.Executable) != 0 {
		return filepath.Abs(c.Executable)
	}
	return osext.Executable()
}
