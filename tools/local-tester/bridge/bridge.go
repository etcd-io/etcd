// Copyright 2016 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package main is the entry point for the local tester network bridge.
package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

func bridge(conn net.Conn, remoteAddr string) {
	outconn, err := net.Dial("tcp", flag.Args()[1])
	if err != nil {
		log.Println("oops:", err)
		return
	}
	log.Printf("bridging %v <-> %v\n", outconn.LocalAddr(), outconn.RemoteAddr())
	go io.Copy(conn, outconn)
	io.Copy(outconn, conn)
}

func blackhole(conn net.Conn) {
	log.Printf("blackholing connection %v <-> %v\n", conn.LocalAddr(), conn.RemoteAddr())
	io.Copy(ioutil.Discard, conn)
	conn.Close()
}

func readRemoteOnly(conn net.Conn, remoteAddr string) {
	outconn, err := net.Dial("tcp", flag.Args()[1])
	if err != nil {
		log.Println("oops:", err)
		return
	}
	log.Printf("one way %v <- %v\n", outconn.LocalAddr(), outconn.RemoteAddr())
	io.Copy(conn, outconn)
}

func writeRemoteOnly(conn net.Conn, remoteAddr string) {
	outconn, err := net.Dial("tcp", flag.Args()[1])
	if err != nil {
		log.Println("oops:", err)
		return
	}
	log.Printf("one way %v -> %v\n", outconn.LocalAddr(), outconn.RemoteAddr())
	io.Copy(outconn, conn)
}

func randCopy(conn net.Conn, outconn net.Conn) {
	for rand.Intn(10) > 0 {
		b := make([]byte, 4096)
		n, err := outconn.Read(b)
		if err != nil {
			return
		}
		_, err = conn.Write(b[:n])
		if err != nil {
			return
		}
	}
}

func randomBlackhole(conn net.Conn, remoteAddr string) {
	outconn, err := net.Dial("tcp", flag.Args()[1])
	if err != nil {
		log.Println("oops:", err)
		return
	}
	log.Printf("random blackhole: connection %v <-/-> %v\n", outconn.LocalAddr(), outconn.RemoteAddr())

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		randCopy(conn, outconn)
		wg.Done()
	}()
	go func() {
		randCopy(outconn, conn)
		wg.Done()
	}()
	wg.Wait()
	conn.Close()
	outconn.Close()
}

type config struct {
	delayAccept bool
	resetListen bool

	connFaultRate   float64
	immediateClose  bool
	blackhole       bool
	timeClose       bool
	writeRemoteOnly bool
	readRemoteOnly  bool
	randomBlackhole bool
}

type acceptFaultFunc func()
type connFaultFunc func(net.Conn)

func main() {
	var cfg config

	flag.BoolVar(&cfg.delayAccept, "delay-accept", true, "delays accepting new connections")
	flag.BoolVar(&cfg.resetListen, "reset-listen", true, "resets the listening port")

	flag.Float64Var(&cfg.connFaultRate, "conn-fault-rate", 0.25, "rate of faulty connections")
	flag.BoolVar(&cfg.immediateClose, "immediate-close", true, "close after accept")
	flag.BoolVar(&cfg.blackhole, "blackhole", true, "reads nothing, writes go nowhere")
	flag.BoolVar(&cfg.timeClose, "time-close", true, "close after random time")
	flag.BoolVar(&cfg.writeRemoteOnly, "write-remote-only", true, "only write, no read")
	flag.BoolVar(&cfg.readRemoteOnly, "read-remote-only", true, "only read, no write")
	flag.BoolVar(&cfg.randomBlackhole, "random-blockhole", true, "blackhole after data xfer")
	flag.Parse()

	lAddr := flag.Args()[0]
	fwdAddr := flag.Args()[1]
	log.Println("listening on ", lAddr)
	log.Println("forwarding to ", fwdAddr)
	l, err := net.Listen("tcp", lAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	acceptFaults := []acceptFaultFunc{func() {}}
	if cfg.delayAccept {
		f := func() {
			log.Println("delaying accept")
			time.Sleep(3 * time.Second)
		}
		acceptFaults = append(acceptFaults, f)
	}
	if cfg.resetListen {
		f := func() {
			log.Println("reset listen port")
			l.Close()
			newListener, err := net.Listen("tcp", lAddr)
			if err != nil {
				log.Fatal(err)
			}
			l = newListener

		}
		acceptFaults = append(acceptFaults, f)
	}

	connFaults := []connFaultFunc{func(c net.Conn) { bridge(c, fwdAddr) }}
	if cfg.immediateClose {
		f := func(c net.Conn) {
			log.Println("terminating connection immediately")
			c.Close()
		}
		connFaults = append(connFaults, f)
	}
	if cfg.blackhole {
		connFaults = append(connFaults, blackhole)
	}
	if cfg.timeClose {
		f := func(c net.Conn) {
			go func() {
				t := time.Duration(rand.Intn(5)+1) * time.Second
				time.Sleep(t)
				log.Printf("killing connection %v <-> %v after %v\n",
					c.LocalAddr(),
					c.RemoteAddr(),
					t)
				c.Close()
			}()
			bridge(c, fwdAddr)
		}
		connFaults = append(connFaults, f)
	}
	if cfg.writeRemoteOnly {
		f := func(c net.Conn) { writeRemoteOnly(c, fwdAddr) }
		connFaults = append(connFaults, f)
	}
	if cfg.readRemoteOnly {
		f := func(c net.Conn) { readRemoteOnly(c, fwdAddr) }
		connFaults = append(connFaults, f)
	}
	if cfg.randomBlackhole {
		f := func(c net.Conn) { randomBlackhole(c, fwdAddr) }
		connFaults = append(connFaults, f)
	}

	for {
		acceptFaults[rand.Intn(len(acceptFaults))]()
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}

		r := rand.Intn(len(connFaults))
		if rand.Intn(100) > int(100.0*cfg.connFaultRate) {
			r = 0
		}
		go connFaults[r](conn)
	}

}
