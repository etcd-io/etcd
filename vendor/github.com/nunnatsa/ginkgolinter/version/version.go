package version

var (
	version = "unknown"
	gitHash = "unknown"
)

func Version() string {
	return version
}

func GitHash() string {
	return gitHash
}
