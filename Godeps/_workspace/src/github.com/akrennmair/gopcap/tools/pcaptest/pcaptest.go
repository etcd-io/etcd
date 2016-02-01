package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/akrennmair/gopcap"
)

func min(x uint32, y uint32) uint32 {
	if x < y {
		return x
	}
	return y
}

func main() {
	var device *string = flag.String("d", "", "device")
	var file *string = flag.String("r", "", "file")
	var expr *string = flag.String("e", "", "filter expression")

	flag.Parse()

	var h *pcap.Pcap
	var err error

	ifs, err := pcap.Findalldevs()
	if len(ifs) == 0 {
		fmt.Printf("Warning: no devices found : %s\n", err)
	} else {
		for i := 0; i < len(ifs); i++ {
			fmt.Printf("dev %d: %s (%s)\n", i+1, ifs[i].Name, ifs[i].Description)
		}
	}

	if *device != "" {
		h, err = pcap.Openlive(*device, 65535, true, 0)
		if h == nil {
			fmt.Printf("Openlive(%s) failed: %s\n", *device, err)
			return
		}
	} else if *file != "" {
		h, err = pcap.Openoffline(*file)
		if h == nil {
			fmt.Printf("Openoffline(%s) failed: %s\n", *file, err)
			return
		}
	} else {
		fmt.Printf("usage: pcaptest [-d <device> | -r <file>]\n")
		return
	}
	defer h.Close()

	fmt.Printf("pcap version: %s\n", pcap.Version())

	if *expr != "" {
		fmt.Printf("Setting filter: %s\n", *expr)
		err := h.Setfilter(*expr)
		if err != nil {
			fmt.Printf("Warning: setting filter failed: %s\n", err)
		}
	}

	for pkt := h.Next(); pkt != nil; pkt = h.Next() {
		fmt.Printf("time: %d.%06d (%s) caplen: %d len: %d\nData:",
			int64(pkt.Time.Second()), int64(pkt.Time.Nanosecond()),
			time.Unix(int64(pkt.Time.Second()), 0).String(), int64(pkt.Caplen), int64(pkt.Len))
		for i := uint32(0); i < pkt.Caplen; i++ {
			if i%32 == 0 {
				fmt.Printf("\n")
			}
			if 32 <= pkt.Data[i] && pkt.Data[i] <= 126 {
				fmt.Printf("%c", pkt.Data[i])
			} else {
				fmt.Printf(".")
			}
		}
		fmt.Printf("\n\n")
	}

}
