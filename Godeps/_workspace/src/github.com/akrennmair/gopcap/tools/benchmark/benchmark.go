package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/akrennmair/gopcap"
)

func main() {
	var filename *string = flag.String("file", "", "filename")
	var decode *bool = flag.Bool("d", false, "If true, decode each packet")
	var cpuprofile *string = flag.String("cpuprofile", "", "filename")

	flag.Parse()

	h, err := pcap.Openoffline(*filename)
	if err != nil {
		fmt.Printf("Couldn't create pcap reader: %v", err)
	}

	if *cpuprofile != "" {
		if out, err := os.Create(*cpuprofile); err == nil {
			pprof.StartCPUProfile(out)
			defer func() {
				pprof.StopCPUProfile()
				out.Close()
			}()
		} else {
			panic(err)
		}
	}

	i, nilPackets := 0, 0
	start := time.Now()
	for pkt, code := h.NextEx(); code != -2; pkt, code = h.NextEx() {
		if pkt == nil {
			nilPackets++
		} else if *decode {
			pkt.Decode()
		}
		i++
	}
	duration := time.Since(start)
	fmt.Printf("Took %v to process %v packets, %v per packet, %d nil packets\n", duration, i, duration/time.Duration(i), nilPackets)
}
