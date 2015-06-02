package capnslog

import (
	"log"
)

func init() {
	pkg := NewPackageLogger("log", "")
	w := packageWriter{pkg}
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(w)
}

type packageWriter struct {
	pl *PackageLogger
}

func (p packageWriter) Write(b []byte) (int, error) {
	if p.pl.level < INFO {
		return 0, nil
	}
	p.pl.internalLog(calldepth+2, INFO, string(b))
	return len(b), nil
}
