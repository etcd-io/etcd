# Gexpect

Gexpect is a pure golang expect-like module.

It makes it simpler to create and control other terminal applications.

	child, err := gexpect.Spawn("python")
	if err != nil {
		panic(err)
	}
	child.Expect(">>>")
	child.SendLine("print 'Hello World'")
	child.Interact()
	child.Close()

## Examples

`Spawn` handles the argument parsing from a string

	child.Spawn("/bin/sh -c 'echo \"my complicated command\" | tee log | cat > log2'")
	child.ReadLine() // ReadLine() (string, error)
	child.ReadUntil(' ') // ReadUntil(delim byte) ([]byte, error)

`ReadLine`, `ReadUntil` and `SendLine` send strings from/to `stdout/stdin` respectively

	child := gexpect.Spawn("cat")
	child.SendLine("echoing process_stdin") //  SendLine(command string) (error)
	msg, _ := child.ReadLine() // msg = echoing process_stdin

`Wait` and `Close` allow for graceful and ungraceful termination.

	child.Wait() // Waits until the child terminates naturally.
	child.Close() // Sends a kill command

`AsyncInteractChannels` spawns two go routines to pipe into and from `stdout`/`stdin`, allowing for some usecases to be a little simpler.

	child := gexpect.spawn("sh")
	sender, reciever := child.AsyncInteractChannels()
	sender <- "echo Hello World\n" // Send to stdin
	line, open := <- reciever // Recieve a line from stdout/stderr
	// When the subprocess stops (e.g. with child.Close()) , receiver is closed
	if open {
		fmt.Printf("Received %s", line)]
	}

`ExpectRegex` uses golang's internal regex engine to wait until a match from the process with the given regular expression (or an error on process termination with no match).

	child := gexpect.Spawn("echo accb")	
	match, _ := child.ExpectRegex("a..b")
	// (match=true)

`ExpectRegexFind` allows for groups to be extracted from process stdout. The first element is an array of containing the total matched text, followed by each subexpression group match.

	child := gexpect.Spawn("echo 123 456 789")
	result, _ := child.ExpectRegexFind("\d+ (\d+) (\d+)")
	// result = []string{"123 456 789", "456", "789"}

See `gexpect_test.go` and the `examples` folder for full syntax

## Credits

	github.com/kballard/go-shellquote	
	github.com/kr/pty
	KMP Algorithm: "http://blog.databigbang.com/searching-for-substrings-in-streams-a-slight-modification-of-the-knuth-morris-pratt-algorithm-in-haxe/"