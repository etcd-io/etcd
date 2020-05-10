// Package socket is client of `golang.org/x/tools/playground/socket`.
package socket

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"

	"golang.org/x/net/websocket"
	"golang.org/x/tools/playground/socket"
)

var random *rand.Rand

func init() {
	random = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// Client represents client of go playground socket server.
type Client struct {
	Conn *websocket.Conn

	Stdout io.Writer
	Stderr io.Writer
}

// Run runs code on connected go playground websocket server.
func (c *Client) Run(code string) error {
	stdout, stderr := c.Stdout, c.Stderr
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	id := fmt.Sprintf("goplay-%d", random.Int())

	mes := &socket.Message{
		Id:   id,
		Kind: "run",
		Body: code,
	}

	if err := json.NewEncoder(c.Conn).Encode(mes); err != nil {
		return err
	}

	scanner := bufio.NewScanner(c.Conn)
	for scanner.Scan() {
		var m socket.Message
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			return err
		}
		if m.Id != id {
			continue
		}
		if m.Kind == "end" {
			break
		}
		w := stderr
		if m.Kind == "stdout" {
			w = stdout
		}
		fmt.Fprint(w, m.Body)
	}
	return nil
}
