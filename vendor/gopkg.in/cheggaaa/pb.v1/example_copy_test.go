package pb_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/cheggaaa/pb.v1"
)

func Example_copy() {
	// check args
	if len(os.Args) < 3 {
		printUsage()
		return
	}
	sourceName, destName := os.Args[1], os.Args[2]

	// check source
	var source io.Reader
	var sourceSize int64
	if strings.HasPrefix(sourceName, "http://") {
		// open as url
		resp, err := http.Get(sourceName)
		if err != nil {
			fmt.Printf("Can't get %s: %v\n", sourceName, err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Server return non-200 status: %v\n", resp.Status)
			return
		}
		i, _ := strconv.Atoi(resp.Header.Get("Content-Length"))
		sourceSize = int64(i)
		source = resp.Body
	} else {
		// open as file
		s, err := os.Open(sourceName)
		if err != nil {
			fmt.Printf("Can't open %s: %v\n", sourceName, err)
			return
		}
		defer s.Close()
		// get source size
		sourceStat, err := s.Stat()
		if err != nil {
			fmt.Printf("Can't stat %s: %v\n", sourceName, err)
			return
		}
		sourceSize = sourceStat.Size()
		source = s
	}

	// create dest
	dest, err := os.Create(destName)
	if err != nil {
		fmt.Printf("Can't create %s: %v\n", destName, err)
		return
	}
	defer dest.Close()

	// create bar
	bar := pb.New(int(sourceSize)).SetUnits(pb.U_BYTES).SetRefreshRate(time.Millisecond * 10)
	bar.ShowSpeed = true
	bar.Start()

	// create proxy reader
	reader := bar.NewProxyReader(source)

	// and copy from reader
	io.Copy(dest, reader)
	bar.Finish()
}

func printUsage() {
	fmt.Println("copy [source file or url] [dest file]")
}
