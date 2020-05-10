package socket

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"

	"golang.org/x/net/websocket"
	"golang.org/x/tools/playground/socket"
)

const origin = "http://127.0.0.1/"

const code = `
package main

import "fmt"

func main() {
	fmt.Println("Hello, 世界!")
}
`

func ExampleClient() {
	// Serve websocket playground server
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		log.Fatal(err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	mu := http.NewServeMux()

	u, err := url.Parse(fmt.Sprintf("http://%s", l.Addr()))
	if err != nil {
		log.Fatal(err)
	}
	mu.Handle("/", socket.NewHandler(u))
	s := http.Server{Handler: mu}
	go s.Serve(l)

	url := fmt.Sprintf("ws://%s/", l.Addr())

	config, err := websocket.NewConfig(url, origin)
	if err != nil {
		log.Fatal(err)
	}
	ws, err := websocket.DialConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	cli := &Client{Conn: ws}
	cli.Run(code)
	// Output:
	// Hello, 世界!
}
