package runtime

import (
	"io/ioutil"
	"syscall"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codahale/metrics"
)

func getFDLimit() (uint64, error) {
	var rlimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
		return 0, err
	}
	return rlimit.Cur, nil
}

func getFDUsage() (uint64, error) {
	fds, err := ioutil.ReadDir("/proc/self/fd")
	if err != nil {
		return 0, err
	}
	return uint64(len(fds)), nil
}

func init() {
	metrics.Gauge("FileDescriptors.Max").SetFunc(func() int64 {
		v, err := getFDLimit()
		if err != nil {
			return 0
		}
		return int64(v)
	})

	metrics.Gauge("FileDescriptors.Used").SetFunc(func() int64 {
		v, err := getFDUsage()
		if err != nil {
			return 0
		}
		return int64(v)
	})
}
