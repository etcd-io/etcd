package store

import (
	"fmt"
	"time"

	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
)

// A lookup of factories by version.
var factories = make(map[int]CommandFactory)
var minVersion, maxVersion int

// The CommandFactory provides a way to create different types of commands
// depending on the current version of the store.
type CommandFactory interface {
	Version() int
	CreateUpgradeCommand() raft.Command
	CreateSetCommand(key string, dir bool, value string, expireTime time.Time) raft.Command
	CreateCreateCommand(key string, dir bool, value string, expireTime time.Time, unique bool) raft.Command
	CreateUpdateCommand(key string, value string, expireTime time.Time) raft.Command
	CreateDeleteCommand(key string, dir, recursive bool) raft.Command
	CreateCompareAndSwapCommand(key string, value string, prevValue string,
		prevIndex uint64, expireTime time.Time) raft.Command
	CreateCompareAndDeleteCommand(key string, prevValue string, prevIndex uint64) raft.Command
	CreateSyncCommand(now time.Time) raft.Command
	CreateGetCommand(key string, recursive, sorted bool) raft.Command
}

// RegisterCommandFactory adds a command factory to the global registry.
func RegisterCommandFactory(factory CommandFactory) {
	version := factory.Version()

	if GetCommandFactory(version) != nil {
		panic(fmt.Sprintf("Command factory already registered for version: %d", factory.Version()))
	}

	factories[version] = factory

	// Update compatibility versions.
	if minVersion == 0 || version > minVersion {
		minVersion = version
	}
	if maxVersion == 0 || version > maxVersion {
		maxVersion = version
	}
}

// GetCommandFactory retrieves a command factory for a given command version.
func GetCommandFactory(version int) CommandFactory {
	return factories[version]
}

// MinVersion returns the minimum compatible store version.
func MinVersion() int {
	return minVersion
}

// MaxVersion returns the maximum compatible store version.
func MaxVersion() int {
	return maxVersion
}
