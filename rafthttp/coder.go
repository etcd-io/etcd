package rafthttp

import "github.com/coreos/etcd/raft/raftpb"

type encoder interface {
	// encode encodes the given message to an output stream.
	encode(m raftpb.Message) error
}

type decoder interface {
	// decode decodes the message from an input stream.
	decode() (raftpb.Message, error)
}
