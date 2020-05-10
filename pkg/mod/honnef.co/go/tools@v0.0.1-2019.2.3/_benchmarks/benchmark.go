// +build ignore

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os/exec"
	"strings"
)

const theBigMerge = "0a9027c2bab9ca0d25a5db0f906fd1793774fd67"
const N = 10

func isAfterMerge(sha string) bool {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", theBigMerge, sha)
	err := cmd.Run()
	if err == nil {
		return true
	}
	_ = err.(*exec.ExitError)
	return false
}

func checkout(sha string) {
	cmd := exec.Command("git", "checkout", "-q", sha)
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

func build(tool string) {
	err := exec.Command("go", "build", "-o", "/tmp/"+tool, "honnef.co/go/tools/cmd/"+tool).Run()
	if err != nil {
		panic(err)
	}
}

func run(tool, target string) (time, mem string) {
	cmd := exec.Command("/usr/bin/time", "-f", "%e %M", "/tmp/"+tool, target)
	out, _ := cmd.CombinedOutput()
	lines := bytes.Split(out, []byte("\n"))
	res := string(lines[len(lines)-2])
	fields := strings.Split(res, " ")
	return fields[0], fields[1]
}

func main() {
	var (
		shas    string
		targets string
		version string
	)
	flag.StringVar(&shas, "shas", "HEAD", "")
	flag.StringVar(&targets, "targets", "std", "")
	flag.StringVar(&version, "version", "unknown", "")
	flag.Parse()

	for _, sha := range strings.Split(shas, ",") {
		tool := "megacheck"
		if isAfterMerge(sha) {
			tool = "staticcheck"
		}
		checkout(sha)
		build(tool)

		for _, target := range strings.Split(targets, ",") {
			for i := 0; i < N; i++ {
				time, mem := run(tool, target)
				fmt.Printf("%s %s %s %s %s\n", sha, version, target, time, mem)
			}
		}
	}
}
