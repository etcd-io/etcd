package goenv

// General-purpose environment variables.
// Reference: https://github.com/golang/go/blob/0afd7e85e5d7154161770f06a17d09bf1ffa3e94/src/cmd/go/internal/help/helpdoc.go#L490
const (
	// GCCGO The gccgo command to run for 'go build -compiler=gccgo'.
	GCCGO = "GCCGO"
	// GO111MODULE Controls whether the go command runs in module-aware mode or GOPATH mode.
	// May be "off", "on", or "auto".
	// See https://golang.org/ref/mod#mod-commands.
	GO111MODULE = "GO111MODULE"
	// GOARCH The architecture, or processor, for which to compile code.
	// Examples are amd64, 386, arm, ppc64.
	GOARCH = "GOARCH"
	// GOAUTH Controls authentication for go-import and HTTPS module mirror interactions.
	// See 'go help goauth'.
	GOAUTH = "GOAUTH"
	// GOBIN The directory where 'go install' will install a command.
	GOBIN = "GOBIN"
	// GOCACHE The directory where the go command will store cached
	// information for reuse in future builds.
	GOCACHE = "GOCACHE"
	// GOCACHEPROG A command (with optional space-separated flags) that implements an
	// external go command build cache.
	// See 'go doc cmd/go/internal/cacheprog'.
	GOCACHEPROG = "GOCACHEPROG"
	// GODEBUG Enable various debugging facilities. See https://go.dev/doc/godebug
	// for details.
	GODEBUG = "GODEBUG"
	// GOENV The location of the Go environment configuration file.
	// Cannot be set using 'go env -w'.
	// Setting GOENV=off in the environment disables the use of the
	// default configuration file.
	GOENV = "GOENV"
	// GOFLAGS A space-separated list of -flag=value settings to apply
	// to go commands by default, when the given flag is known by
	// the current command. Each entry must be a standalone flag.
	// Because the entries are space-separated, flag values must
	// not contain spaces. Flags listed on the command line
	// are applied after this list and therefore override it.
	GOFLAGS = "GOFLAGS"
	// GOINSECURE Comma-separated list of glob patterns (in the syntax of Go's path.Match)
	// of module path prefixes that should always be fetched in an insecure
	// manner. Only applies to dependencies that are being fetched directly.
	// GOINSECURE does not disable checksum database validation. GOPRIVATE or
	// GONOSUMDB may be used to achieve that.
	GOINSECURE = "GOINSECURE"
	// GOMODCACHE The directory where the go command will store downloaded modules.
	GOMODCACHE = "GOMODCACHE"
	// GOOS The operating system for which to compile code.
	// Examples are linux, darwin, windows, netbsd.
	GOOS = "GOOS"
	// GOPATH Controls where various files are stored. See: 'go help gopath'.
	GOPATH = "GOPATH"
	// GOPROXY URL of Go module proxy. See https://golang.org/ref/mod#environment-variables
	// and https://golang.org/ref/mod#module-proxy for details.
	GOPROXY = "GOPROXY"
	// GOROOT The root of the go tree.
	GOROOT = "GOROOT"
	// GOSUMDB The name of checksum database to use and optionally its public key and
	// URL. See https://golang.org/ref/mod#authenticating.
	GOSUMDB = "GOSUMDB"
	// GOTMPDIR The directory where the go command will write
	// temporary source files, packages, and binaries.
	GOTMPDIR = "GOTMPDIR"
	// GOTOOLCHAIN Controls which Go toolchain is used. See https://go.dev/doc/toolchain.
	GOTOOLCHAIN = "GOTOOLCHAIN"
	// GOVCS Lists version control commands that may be used with matching servers.
	// See 'go help vcs'.
	GOVCS = "GOVCS"
	// GOWORK In module aware mode, use the given go.work file as a workspace file.
	// By default or when GOWORK is "auto", the go command searches for a
	// file named go.work in the current directory and then containing directories
	// until one is found. If a valid go.work file is found, the modules
	// specified will collectively be used as the main modules. If GOWORK
	// is "off", or a go.work file is not found in "auto" mode, workspace
	// mode is disabled.
	GOWORK = "GOWORK"

	// GOPRIVATE Comma-separated list of glob patterns (in the syntax of Go's path.Match)
	// of module path prefixes that should always be fetched directly
	// or that should not be compared against the checksum database.
	// See https://golang.org/ref/mod#private-modules.
	GOPRIVATE = "GOPRIVATE"
	// GONOPROXY Comma-separated list of glob patterns (in the syntax of Go's path.Match)
	// of module path prefixes that should always be fetched directly
	// or that should not be compared against the checksum database.
	// See https://golang.org/ref/mod#private-modules.
	GONOPROXY = "GONOPROXY"
	// GONOSUMDB Comma-separated list of glob patterns (in the syntax of Go's path.Match)
	// of module path prefixes that should always be fetched directly
	// or that should not be compared against the checksum database.
	// See https://golang.org/ref/mod#private-modules.
	GONOSUMDB = "GONOSUMDB"
)

// Environment variables for use with cgo.
// Reference: https://github.com/golang/go/blob/0afd7e85e5d7154161770f06a17d09bf1ffa3e94/src/cmd/go/internal/help/helpdoc.go#L571
const (
	// AR The command to use to manipulate library archives when
	// building with the gccgo compiler.
	// The default is 'ar'.
	AR = "AR"
	// CC The command to use to compile C code.
	CC = "CC"
	// CGO_CFLAGS Flags that cgo will pass to the compiler when compiling
	// C code.
	CGO_CFLAGS = "CGO_CFLAGS"
	// CGO_CFLAGS_ALLOW A regular expression specifying additional flags to allow
	// to appear in #cgo CFLAGS source code directives.
	// Does not apply to the CGO_CFLAGS environment variable.
	CGO_CFLAGS_ALLOW = "CGO_CFLAGS_ALLOW"
	// CGO_CFLAGS_DISALLOW A regular expression specifying flags that must be disallowed
	// from appearing in #cgo CFLAGS source code directives.
	// Does not apply to the CGO_CFLAGS environment variable.
	CGO_CFLAGS_DISALLOW = "CGO_CFLAGS_DISALLOW"
	// CGO_ENABLED Whether the cgo command is supported. Either 0 or 1.
	CGO_ENABLED = "CGO_ENABLED"
	// CXX The command to use to compile C++ code.
	CXX = "CXX"
	// FC The command to use to compile Fortran code.
	FC = "FC"
	// PKG_CONFIG Path to pkg-config tool.
	PKG_CONFIG = "PKG_CONFIG"

	// CGO_CPPFLAGS Like CGO_CFLAGS, CGO_CFLAGS_ALLOW, and CGO_CFLAGS_DISALLOW,
	// but for the C preprocessor.
	CGO_CPPFLAGS = "CGO_CPPFLAGS"
	// CGO_CPPFLAGS_ALLOW Like CGO_CFLAGS, CGO_CFLAGS_ALLOW, and CGO_CFLAGS_DISALLOW,
	// but for the C preprocessor.
	CGO_CPPFLAGS_ALLOW = "CGO_CPPFLAGS_ALLOW"
	// CGO_CPPFLAGS_DISALLOW Like CGO_CFLAGS, CGO_CFLAGS_ALLOW, and CGO_CFLAGS_DISALLOW,
	// but for the C preprocessor.
	CGO_CPPFLAGS_DISALLOW = "CGO_CPPFLAGS_DISALLOW"

	// CGO_CXXFLAGS Like CGO_CFLAGS, CGO_CFLAGS_ALLOW, and CGO_CFLAGS_DISALLOW,
	// but for the C++ compiler.
	CGO_CXXFLAGS = "CGO_CXXFLAGS"
	// CGO_CXXFLAGS_ALLOW Like CGO_CFLAGS, CGO_CFLAGS_ALLOW, and CGO_CFLAGS_DISALLOW,
	// but for the C++ compiler.
	CGO_CXXFLAGS_ALLOW = "CGO_CXXFLAGS_ALLOW"
	// CGO_CXXFLAGS_DISALLOW Like CGO_CFLAGS, CGO_CFLAGS_ALLOW, and CGO_CFLAGS_DISALLOW,
	// but for the C++ compiler.
	CGO_CXXFLAGS_DISALLOW = "CGO_CXXFLAGS_DISALLOW"

	// CGO_FFLAGS Like CGO_CFLAGS, CGO_CFLAGS_ALLOW, and CGO_CFLAGS_DISALLOW,
	// but for the Fortran compiler.
	CGO_FFLAGS = "CGO_FFLAGS"
	// CGO_FFLAGS_ALLOW Like CGO_CFLAGS, CGO_CFLAGS_ALLOW, and CGO_CFLAGS_DISALLOW,
	// but for the Fortran compiler.
	CGO_FFLAGS_ALLOW = "CGO_FFLAGS_ALLOW"
	// CGO_FFLAGS_DISALLOW Like CGO_CFLAGS, CGO_CFLAGS_ALLOW, and CGO_CFLAGS_DISALLOW,
	// but for the Fortran compiler.
	CGO_FFLAGS_DISALLOW = "CGO_FFLAGS_DISALLOW"

	// CGO_LDFLAGS Like CGO_CFLAGS, CGO_CFLAGS_ALLOW, and CGO_CFLAGS_DISALLOW,
	// but for the linker.
	CGO_LDFLAGS = "CGO_LDFLAGS"
	// CGO_LDFLAGS_ALLOW Like CGO_CFLAGS, CGO_CFLAGS_ALLOW, and CGO_CFLAGS_DISALLOW,
	// but for the linker.
	CGO_LDFLAGS_ALLOW = "CGO_LDFLAGS_ALLOW"
	// CGO_LDFLAGS_DISALLOW Like CGO_CFLAGS, CGO_CFLAGS_ALLOW, and CGO_CFLAGS_DISALLOW,
	// but for the linker.
	CGO_LDFLAGS_DISALLOW = "CGO_LDFLAGS_DISALLOW"
)

// Architecture-specific environment variables.
// Reference: https://github.com/golang/go/blob/0afd7e85e5d7154161770f06a17d09bf1ffa3e94/src/cmd/go/internal/help/helpdoc.go#L611
const (
	// GO386 For GOARCH=386, how to implement floating point instructions.
	// Valid values are sse2 (default), softfloat.
	GO386 = "GO386"
	// GOAMD64 For GOARCH=amd64, the microarchitecture level for which to compile.
	// Valid values are v1 (default), v2, v3, v4.
	// See https://golang.org/wiki/MinimumRequirements#amd64
	GOAMD64 = "GOAMD64"
	// GOARM For GOARCH=arm, the ARM architecture for which to compile.
	// Valid values are 5, 6, 7.
	// When the Go tools are built on an arm system,
	// the default value is set based on what the build system supports.
	// When the Go tools are not built on an arm system
	// (that is, when building a cross-compiler),
	// the default value is 7.
	// The value can be followed by an option specifying how to implement floating point instructions.
	// Valid options are ,softfloat (default for 5) and ,hardfloat (default for 6 and 7).
	GOARM = "GOARM"
	// GOARM64 For GOARCH=arm64, the ARM64 architecture for which to compile.
	// Valid values are v8.0 (default), v8.{1-9}, v9.{0-5}.
	// The value can be followed by an option specifying extensions implemented by target hardware.
	// Valid options are ,lse and ,crypto.
	// Note that some extensions are enabled by default starting from a certain GOARM64 version;
	// for example, lse is enabled by default starting from v8.1.
	GOARM64 = "GOARM64"
	// GOMIPS For GOARCH=mips{,le}, whether to use floating point instructions.
	// Valid values are hardfloat (default), softfloat.
	GOMIPS = "GOMIPS"
	// GOMIPS64 For GOARCH=mips64{,le}, whether to use floating point instructions.
	// Valid values are hardfloat (default), softfloat.
	GOMIPS64 = "GOMIPS64"
	// GOPPC64 For GOARCH=ppc64{,le}, the target ISA (Instruction Set Architecture).
	// Valid values are power8 (default), power9, power10.
	GOPPC64 = "GOPPC64"
	// GORISCV64 For GOARCH=riscv64, the RISC-V user-mode application profile for which
	// to compile. Valid values are rva20u64 (default), rva22u64.
	// See https://github.com/riscv/riscv-profiles/blob/main/src/profiles.adoc
	GORISCV64 = "GORISCV64"
	// GOWASM For GOARCH=wasm, comma-separated list of experimental WebAssembly features to use.
	// Valid values are satconv, signext.
	GOWASM = "GOWASM"
)

// Environment variables for use with code coverage.
// Reference: https://github.com/golang/go/blob/0afd7e85e5d7154161770f06a17d09bf1ffa3e94/src/cmd/go/internal/help/helpdoc.go#L654
const (
	// GOCOVERDIR Directory into which to write code coverage data files
	// generated by running a "go build -cover" binary.
	// Requires that GOEXPERIMENT=coverageredesign is enabled.
	GOCOVERDIR = "GOCOVERDIR"
)

// Special-purpose environment variables.
// Reference: https://github.com/golang/go/blob/0afd7e85e5d7154161770f06a17d09bf1ffa3e94/src/cmd/go/internal/help/helpdoc.go#L661
const (
	// GCCGOTOOLDIR If set, where to find gccgo tools, such as cgo.
	// The default is based on how gccgo was configured.
	GCCGOTOOLDIR = "GCCGOTOOLDIR"
	// GOEXPERIMENT Comma-separated list of toolchain experiments to enable or disable.
	// The list of available experiments may change arbitrarily over time.
	// See GOROOT/src/internal/goexperiment/flags.go for currently valid values.
	// Warning: This variable is provided for the development and testing
	// of the Go toolchain itself. Use beyond that purpose is unsupported.
	GOEXPERIMENT = "GOEXPERIMENT"
	// GOFIPS140 The FIPS-140 cryptography mode to use when building binaries.
	// The default is GOFIPS140=off, which makes no FIPS-140 changes at all.
	// Other values enable FIPS-140 compliance measures and select alternate
	// versions of the cryptography source code.
	// See https://go.dev/security/fips140 for details.
	GOFIPS140 = "GOFIPS140"
	// GO_EXTLINK_ENABLED Whether the linker should use external linking mode
	// when using -linkmode=auto with code that uses cgo.
	// Set to 0 to disable external linking mode, 1 to enable it.
	GO_EXTLINK_ENABLED = "GO_EXTLINK_ENABLED"
	// GIT_ALLOW_PROTOCOL Defined by Git. A colon-separated list of schemes that are allowed
	// to be used with git fetch/clone. If set, any scheme not explicitly
	// mentioned will be considered insecure by 'go get'.
	// Because the variable is defined by Git, the default value cannot
	// be set using 'go env -w'.
	GIT_ALLOW_PROTOCOL = "GIT_ALLOW_PROTOCOL"
)

// Additional information available from 'go env' but not read from the environment.
// Reference: https://github.com/golang/go/blob/0afd7e85e5d7154161770f06a17d09bf1ffa3e94/src/cmd/go/internal/help/helpdoc.go#L689
const (
	// GOEXE The executable file name suffix (".exe" on Windows, "" on other systems).
	GOEXE = "GOEXE"
	// GOGCCFLAGS A space-separated list of arguments supplied to the CC command.
	GOGCCFLAGS = "GOGCCFLAGS"
	// GOHOSTARCH The architecture (GOARCH) of the Go toolchain binaries.
	GOHOSTARCH = "GOHOSTARCH"
	// GOHOSTOS The operating system (GOOS) of the Go toolchain binaries.
	GOHOSTOS = "GOHOSTOS"
	// GOMOD The absolute path to the go.mod of the main module.
	// If module-aware mode is enabled, but there is no go.mod, GOMOD will be
	// os.DevNull ("/dev/null" on Unix-like systems, "NUL" on Windows).
	// If module-aware mode is disabled, GOMOD will be the empty string.
	GOMOD = "GOMOD"
	// GOTELEMETRY The current Go telemetry mode ("off", "local", or "on").
	// See "go help telemetry" for more information.
	GOTELEMETRY = "GOTELEMETRY"
	// GOTELEMETRYDIR The directory Go telemetry data is written is written to.
	GOTELEMETRYDIR = "GOTELEMETRYDIR"
	// GOTOOLDIR The directory where the go tools (compile, cover, doc, etc...) are installed.
	GOTOOLDIR = "GOTOOLDIR"
	// GOVERSION The version of the installed Go tree, as reported by runtime.Version.
	GOVERSION = "GOVERSION"
)
