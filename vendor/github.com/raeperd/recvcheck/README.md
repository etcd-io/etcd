# recvcheck
[![.github/workflows/build.yaml](https://github.com/raeperd/recvcheck/actions/workflows/build.yaml/badge.svg)](https://github.com/raeperd/recvcheck/actions/workflows/build.yaml) [![Go Report Card](https://goreportcard.com/badge/github.com/raeperd/recvcheck)](https://goreportcard.com/report/github.com/raeperd/recvcheck)   
Golang linter for check receiver type in method 

## Motivation
From [Go Wiki: Go Code Review Comments - The Go Programming Language](https://go.dev/wiki/CodeReviewComments#receiver-type)
> Don’t mix receiver types. Choose either pointers or struct types for all available method

Following code from [Dave Cheney](https://dave.cheney.net/2015/11/18/wednesday-pop-quiz-spot-the-race) causes data race. Could you find it?
This linter does it for you.

```go
package main

import (
        "fmt"
        "time"
)

type RPC struct {
        result int
        done   chan struct{}
}

func (rpc *RPC) compute() {
        time.Sleep(time.Second) // strenuous computation intensifies
        rpc.result = 42
        close(rpc.done)
}

func (RPC) version() int {
        return 1 // never going to need to change this
}

func main() {
        rpc := &RPC{done: make(chan struct{})}

        go rpc.compute()         // kick off computation in the background
        version := rpc.version() // grab some other information while we're waiting
        <-rpc.done               // wait for computation to finish
        result := rpc.result

        fmt.Printf("RPC computation complete, result: %d, version: %d\n", result, version)
}
```

## References
- [Is there a way to detect following data race code using golangci-lint or other linter?? · golangci/golangci-lint · Discussion #5006](https://github.com/golangci/golangci-lint/discussions/5006)
    - [Wednesday pop quiz: spot the race | Dave Cheney](https://dave.cheney.net/2015/11/18/wednesday-pop-quiz-spot-the-race)



