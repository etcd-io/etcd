package knowledge

var KnownGOOS = map[string]struct{}{
	"aix":       {},
	"android":   {},
	"darwin":    {},
	"dragonfly": {},
	"freebsd":   {},
	"hurd":      {},
	"illumos":   {},
	"ios":       {},
	"js":        {},
	"linux":     {},
	"netbsd":    {},
	"openbsd":   {},
	"plan9":     {},
	"solaris":   {},
	"wasip1":    {},
	"windows":   {},
}

var KnownGOARCH = map[string]struct{}{
	"386":      {},
	"amd64":    {},
	"arm":      {},
	"arm64":    {},
	"loong64":  {},
	"mips":     {},
	"mipsle":   {},
	"mips64":   {},
	"mips64le": {},
	"ppc64":    {},
	"ppc64le":  {},
	"riscv64":  {},
	"s390x":    {},
	"sparc64":  {},
	"wasm":     {},
}
