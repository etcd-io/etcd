package main

import (
	"fmt"
	"os"

	"gotest.tools/gotestsum/cmd"
	"gotest.tools/gotestsum/cmd/tool/matrix"
	"gotest.tools/gotestsum/cmd/tool/slowest"
	"gotest.tools/gotestsum/internal/log"
)

func main() {
	err := route(os.Args)
	switch {
	case err == nil:
		return
	case cmd.IsExitCoder(err):
		// go test should already report the error to stderr, exit with
		// the same status code
		os.Exit(cmd.ExitCodeWithDefault(err))
	default:
		log.Error(err.Error())
		os.Exit(3)
	}
}

func route(args []string) error {
	name := args[0]
	next, rest := nextArg(args[1:])
	switch next {
	case "help", "?":
		return cmd.Run(name, []string{"--help"})
	case "tool":
		return toolRun(name+" "+next, rest)
	default:
		return cmd.Run(name, args[1:])
	}
}

// nextArg splits args into the next positional argument and any remaining args.
func nextArg(args []string) (string, []string) {
	switch len(args) {
	case 0:
		return "", nil
	case 1:
		return args[0], nil
	default:
		return args[0], args[1:]
	}
}

func toolRun(name string, args []string) error {
	usage := func(name string) string {
		return fmt.Sprintf(`Usage: %[1]s COMMAND [flags]

Commands:
    %[1]s slowest      find or skip the slowest tests
    %[1]s ci-matrix    use previous test runtime to place packages into optimal buckets

Use '%[1]s COMMAND --help' for command specific help.
`, name)
	}

	next, rest := nextArg(args)
	switch next {
	case "", "help", "?":
		fmt.Println(usage(name))
		return nil
	case "slowest":
		return slowest.Run(name+" "+next, rest)
	case "ci-matrix":
		return matrix.Run(name+" "+next, rest)
	default:
		fmt.Fprintln(os.Stderr, usage(name))
		return fmt.Errorf("invalid command: %v %v", name, next)
	}
}
