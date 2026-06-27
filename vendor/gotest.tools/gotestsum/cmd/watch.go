package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"

	"gotest.tools/gotestsum/internal/filewatcher"
	"gotest.tools/gotestsum/testjson"
)

func runWatcher(opts *options) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := &watchRuns{opts: *opts}
	return filewatcher.Watch(ctx, opts.packages, opts.watchClear, w.run)
}

type watchRuns struct {
	opts     options
	prevExec *testjson.Execution
}

func (w *watchRuns) run(event filewatcher.Event) error {
	if event.Debug {
		path, cleanup, err := delveInitFile(w.prevExec)
		if err != nil {
			return fmt.Errorf("failed to write delve init file: %w", err)
		}
		defer cleanup()
		o := delveOpts{
			pkgPath:      event.PkgPath,
			args:         w.opts.args,
			initFilePath: path,
		}
		if err := runDelve(o); !IsExitCoder(err) {
			return fmt.Errorf("delve failed: %w", err)
		}
		return nil
	}

	var dir string
	if w.opts.watchChdir {
		dir, event.PkgPath = event.PkgPath, "./"
	}

	opts := w.opts // shallow copy opts
	opts.packages = append([]string{}, opts.packages...)
	opts.packages = append(opts.packages, event.PkgPath)
	opts.packages = append(opts.packages, event.Args...)

	var err error
	if w.prevExec, err = runSingle(&opts, dir); !IsExitCoder(err) {
		return err
	}
	return nil
}

// runSingle is similar to run. It doesn't support rerun-fails. It may be
// possible to share runSingle with run, but the defer close on the handler
// would require at least 3 return values, so for now it is a copy.
func runSingle(opts *options, dir string) (*testjson.Execution, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	goTestProc, err := startGoTestFn(ctx, dir, goTestCmdArgs(opts, rerunOpts{}))
	if err != nil {
		return nil, err
	}

	handler, err := newEventHandler(opts)
	if err != nil {
		return nil, err
	}
	defer handler.Close() //nolint:errcheck
	cfg := testjson.ScanConfig{
		Stdout:  goTestProc.stdout,
		Stderr:  goTestProc.stderr,
		Handler: handler,
		Stop:    cancel,
	}
	exec, err := testjson.ScanTestOutput(cfg)
	handler.Flush()
	if err != nil {
		return exec, finishRun(opts, exec, err)
	}
	err = goTestProc.cmd.Wait()
	return exec, finishRun(opts, exec, err)
}

func delveInitFile(exec *testjson.Execution) (string, func(), error) {
	fh, err := os.CreateTemp("", "gotestsum-delve-init")
	if err != nil {
		return "", nil, err
	}
	remove := func() {
		os.Remove(fh.Name()) //nolint:errcheck
	}

	buf := bufio.NewWriter(fh)
	for _, tc := range exec.Failed() {
		fmt.Fprintf(buf, "break %s\n", tc.Test.Name())
	}
	buf.WriteString("continue\n")
	if err := buf.Flush(); err != nil {
		remove()
		return "", nil, err
	}
	return fh.Name(), remove, nil
}

type delveOpts struct {
	pkgPath      string
	args         []string
	initFilePath string
}

func runDelve(opts delveOpts) error {
	pkg := opts.pkgPath
	args := []string{"dlv", "test", "--wd", pkg}
	args = append(args, "--output", "gotestsum-watch-debug.test")
	args = append(args, "--init", opts.initFilePath)
	args = append(args, pkg, "--")
	args = append(args, opts.args...)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
