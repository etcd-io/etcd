// +build windows

package etcdmain

// TODO(barakmich): So because file locking on Windows is untested, the
// temporary fix is to default to unlimited snapshots and WAL files, with manual
// removal. Perhaps not the most elegant solution, but it's at least safe and
// we'd totally love a PR to fix the story around locking.
const (
	defaultMaxSnapshots = 0
	defaultMaxWALs      = 0
)
