package etcd

import (
	"github.com/ccding/go-logging/logging"
)

var logger, _ = logging.SimpleLogger("go-etcd")

func init() {
	logger.SetLevel(logging.FATAL)
}

func OpenDebug() {
	logger.SetLevel(logging.NOTSET)
}

func CloseDebug() {
	logger.SetLevel(logging.FATAL)
}
