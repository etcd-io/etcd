package command

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

// NewExecWatchCommand returns the CLI command for "exec-watch".
func NewExecWatchCommand() cli.Command {
	return cli.Command{
		Name:  "exec-watch",
		Usage: "watch a key for changes and exec an executable",
		Flags: []cli.Flag{
			cli.IntFlag{Name: "after-index", Value: 0, Usage: "watch after the given index"},
			cli.BoolFlag{Name: "recursive", Usage: "watch all values for key and child keys"},
			cli.BoolFlag{Name: "synthesize-set", Usage: "synthesize fake \"set\" actions for pre-existing keys"},
		},
		Action: func(c *cli.Context) {
			handleKey(c, execWatchCommandFunc)
		},
	}
}

// execWatchCommandFunc executes the "exec-watch" command.
func execWatchCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	_ = io.Copy
	_ = exec.Command
	args := c.Args()
	argsLen := len(args)

	if argsLen < 2 {
		return nil, errors.New("Key and command to exec required")
	}

	key := args[argsLen-1]
	cmdArgs := args[:argsLen-1]

	index := uint64(0)
	if c.Int("after-index") != 0 {
		index = uint64(c.Int("after-index")) + 1
		key = args[0]
		cmdArgs = args[2:]
	}

	recursive := c.Bool("recursive")
	if recursive {
		key = args[0]
		cmdArgs = args[2:]
	}

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	stop := make(chan bool)

	go func() {
		<-sigch
		stop <- true
		os.Exit(0)
	}()

	receiver := make(chan *etcd.Response)
	client.SetConsistency(etcd.WEAK_CONSISTENCY)

	synthesize_set := c.Bool("synthesize-set")
	go func() {
		if synthesize_set {
			resp, err := client.Get(key, false, recursive)
			if err == nil {
				rWalk(resp.Node, func(n *etcd.Node) {
					receiver <- &etcd.Response{Action: "set", Node: n}
				})
				index = resp.EtcdIndex + 1
			}
		}

		client.Watch(key, index, recursive, receiver, stop)
	}()

	for {
		resp := <-receiver
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Env = environResponse(resp, os.Environ())

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
		err = cmd.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
		go io.Copy(os.Stdout, stdout)
		go io.Copy(os.Stderr, stderr)
		cmd.Wait()
	}
}

func rWalk(n *etcd.Node, f func(n *etcd.Node)) {
	if !n.Dir {
		f(n)
	}
	for _, sub_node := range n.Nodes {
		rWalk(sub_node, f)
	}
}

func environResponse(resp *etcd.Response, env []string) []string {
	env = append(env, "ETCD_WATCH_ACTION="+resp.Action)
	env = append(env, "ETCD_WATCH_MODIFIED_INDEX="+fmt.Sprintf("%d", resp.Node.ModifiedIndex))
	env = append(env, "ETCD_WATCH_KEY="+resp.Node.Key)
	env = append(env, "ETCD_WATCH_VALUE="+resp.Node.Value)
	return env
}
