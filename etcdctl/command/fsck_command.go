
package command

import (
	"fmt"
	"log"
	"path"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/wal"
	"github.com/coreos/etcd/wal/walpb"
)

func NewFsckCommand() cli.Command {
	return cli.Command{
		Name:  "fsck",
		Usage: "diagnose an etcd directory in offline condition",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "data-dir", Value: "", Usage: "Path to the etcd data dir"},
			cli.BoolFlag{Name: "verbose", Usage: "be verbose"},
		},
		Action: handleFsck,
	}
}

func handleFsck(c *cli.Context) {
	verbose := c.Bool("verbose")

	snapPath := path.Join(c.String("data-dir"), "member", "snap")
	walPath := path.Join(c.String("data-dir"), "member", "wal")

	ss := snap.New(snapPath)
	snapshot, err := ss.Load()
	if err != nil && err != snap.ErrNoSnapshot {
		log.Fatal(err)
	}

	var walsnap walpb.Snapshot
	if snapshot != nil {
		walsnap.Index, walsnap.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
	}

	w, err := wal.OpenForRead(walPath, walsnap)
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	_, _, _, err = w.ReadAll()
	if err != nil {
		fmt.Errorf("failed to read wal (index: %d, term: %d): %s", err)
	} else {
		if verbose {
			fmt.Printf("WAL is healthy (index: %d, term: %d)\n", walsnap.Index, walsnap.Term)
		}
	}
}
