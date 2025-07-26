package common

import "go.etcd.io/etcd/etcdctl/v3/diagnosis/agent"

type Checker struct {
	agent.GlobalConfig
	Name string
}
