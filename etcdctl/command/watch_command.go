package command

import (
	"errors"
	"os"
	"os/signal"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

// NewWatchCommand returns the CLI command for "watch".
func NewWatchCommand() cli.Command {
	return cli.Command{
		Name:  "watch",
		Usage: "watch a key for changes",
		Flags: []cli.Flag{
			cli.BoolFlag{"forever", "forever watch a key until CTRL+C"},
			cli.IntFlag{"after-index", 0, "watch after the given index"},
			cli.BoolFlag{"recursive", "returns all values for key and child keys"},
		},
		Action: func(c *cli.Context) {
			handleKey(c, watchCommandFunc)
		},
	}
}

// watchCommandFunc executes the "watch" command.
func watchCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	if len(c.Args()) == 0 {
		return nil, errors.New("Key required")
	}
	key := c.Args()[0]
	recursive := c.Bool("recursive")
	forever := c.Bool("forever")

	index := 0
	if c.Int("after-index") != 0 {
		index = c.Int("after-index") + 1
	}

	if forever {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, os.Interrupt)
		stop := make(chan bool)

		go func() {
			<-sigch
			os.Exit(0)
		}()

		receiver := make(chan *etcd.Response)
		errCh := make(chan error, 1)

		go func() {
			_, err := client.Watch(key, uint64(index), recursive, receiver, stop)
			errCh <- err
		}()

		for {
			select {
			case resp := <-receiver:
				printAll(resp, c.GlobalString("output"))
			case err := <-errCh:
				handleError(-1, err)
			}
		}

	} else {
		var resp *etcd.Response
		var err error
		resp, err = client.Watch(key, uint64(index), recursive, nil, nil)

		if err != nil {
			handleError(ErrorFromEtcd, err)
		}

		if err != nil {
			return nil, err
		}
		printAll(resp, c.GlobalString("output"))
	}

	return nil, nil
}
