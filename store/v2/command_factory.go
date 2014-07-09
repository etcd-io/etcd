package v2

import (
	"time"

	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
)

func init() {
	store.RegisterCommandFactory(&CommandFactory{})
}

// CommandFactory provides a pluggable way to create version 2 commands.
type CommandFactory struct {
}

// Version returns the version of this factory.
func (f *CommandFactory) Version() int {
	return 2
}

// CreateUpgradeCommand is a no-op since version 2 is the first version to support store versioning.
func (f *CommandFactory) CreateUpgradeCommand() raft.Command {
	return &raft.NOPCommand{}
}

// CreateSetCommand creates a version 2 command to set a key to a given value in the store.
func (f *CommandFactory) CreateSetCommand(key string, dir bool, value string, expireTime time.Time) raft.Command {
	return &SetCommand{
		Key:        key,
		Value:      value,
		ExpireTime: expireTime,
		Dir:        dir,
	}
}

// CreateCreateCommand creates a version 2 command to create a new key in the store.
func (f *CommandFactory) CreateCreateCommand(key string, dir bool, value string, expireTime time.Time, unique bool) raft.Command {
	return &CreateCommand{
		Key:        key,
		Value:      value,
		ExpireTime: expireTime,
		Unique:     unique,
		Dir:        dir,
	}
}

// CreateUpdateCommand creates a version 2 command to update a key to a given value in the store.
func (f *CommandFactory) CreateUpdateCommand(key string, value string, expireTime time.Time) raft.Command {
	return &UpdateCommand{
		Key:        key,
		Value:      value,
		ExpireTime: expireTime,
	}
}

// CreateDeleteCommand creates a version 2 command to delete a key from the store.
func (f *CommandFactory) CreateDeleteCommand(key string, dir, recursive bool) raft.Command {
	return &DeleteCommand{
		Key:       key,
		Recursive: recursive,
		Dir:       dir,
	}
}

// CreateCompareAndSwapCommand creates a version 2 command to conditionally set a key in the store.
func (f *CommandFactory) CreateCompareAndSwapCommand(key string, value string, prevValue string, prevIndex uint64, expireTime time.Time) raft.Command {
	return &CompareAndSwapCommand{
		Key:        key,
		Value:      value,
		PrevValue:  prevValue,
		PrevIndex:  prevIndex,
		ExpireTime: expireTime,
	}
}

// CreateCompareAndDeleteCommand creates a version 2 command to conditionally delete a key from the store.
func (f *CommandFactory) CreateCompareAndDeleteCommand(key string, prevValue string, prevIndex uint64) raft.Command {
	return &CompareAndDeleteCommand{
		Key:       key,
		PrevValue: prevValue,
		PrevIndex: prevIndex,
	}
}

func (f *CommandFactory) CreateSyncCommand(now time.Time) raft.Command {
	return &SyncCommand{
		Time: time.Now(),
	}
}

func (f *CommandFactory) CreateGetCommand(key string, recursive, sorted bool) raft.Command {
	return &GetCommand{
		Key:       key,
		Recursive: recursive,
		Sorted:    sorted,
	}
}
