package main

// Parses a pcap file, writes it back to disk, then verifies the files
// are the same.
import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/akrennmair/gopcap"
)

var input *string = flag.String("input", "", "input file")
var output *string = flag.String("output", "", "output file")
var decode *bool = flag.Bool("decode", false, "print decoded packets")

func copyPcap(dest, src string) {
	f, err := os.Open(src)
	if err != nil {
		fmt.Printf("couldn't open %q: %v\n", src, err)
		return
	}
	defer f.Close()
	reader, err := pcap.NewReader(bufio.NewReader(f))
	if err != nil {
		fmt.Printf("couldn't create reader: %v\n", err)
		return
	}
	w, err := os.Create(dest)
	if err != nil {
		fmt.Printf("couldn't open %q: %v\n", dest, err)
		return
	}
	defer w.Close()
	buf := bufio.NewWriter(w)
	writer, err := pcap.NewWriter(buf, &reader.Header)
	if err != nil {
		fmt.Printf("couldn't create writer: %v\n", err)
		return
	}
	for {
		pkt := reader.Next()
		if pkt == nil {
			break
		}
		if *decode {
			pkt.Decode()
			fmt.Println(pkt.String())
		}
		writer.Write(pkt)
	}
	buf.Flush()
}

func check(dest, src string) {
	f, err := os.Open(src)
	if err != nil {
		fmt.Printf("couldn't open %q: %v\n", src, err)
		return
	}
	defer f.Close()
	freader := bufio.NewReader(f)

	g, err := os.Open(dest)
	if err != nil {
		fmt.Printf("couldn't open %q: %v\n", src, err)
		return
	}
	defer g.Close()
	greader := bufio.NewReader(g)

	for {
		fb, ferr := freader.ReadByte()
		gb, gerr := greader.ReadByte()

		if ferr == io.EOF && gerr == io.EOF {
			break
		}
		if fb == gb {
			continue
		}
		fmt.Println("FAIL")
		return
	}

	fmt.Println("PASS")
}

func main() {
	flag.Parse()

	copyPcap(*output, *input)
	check(*output, *input)
}
