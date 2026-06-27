package gochecksumtype

type Config struct {
	DefaultSignifiesExhaustive bool
	// IncludeSharedInterfaces in the exhaustiviness check. If true, we do not need to list all concrete structs, as long
	// as the switch statement is exhaustive with respect to interfaces the structs implement.
	IncludeSharedInterfaces bool
}
