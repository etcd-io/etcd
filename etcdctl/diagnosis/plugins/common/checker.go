package common

import (
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Checker carries shared configuration for diagnosis plugins.
// It embeds generic options such as the etcd client configuration,
// resolved endpoints, and command timeout.
type Checker struct {
	Cfg            *clientv3.ConfigSpec
	Endpoints      []string
	CommandTimeout time.Duration
	DbQuotaBytes   int
	Name           string
}
