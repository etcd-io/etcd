package pb_test

import (
	"math/rand"
	"sync"
	"time"

	"gopkg.in/cheggaaa/pb.v1"
)

func Example_multiple() {
	// create bars
	first := pb.New(200).Prefix("First ")
	second := pb.New(200).Prefix("Second ")
	third := pb.New(200).Prefix("Third ")
	// start pool
	pool, err := pb.StartPool(first, second, third)
	if err != nil {
		panic(err)
	}
	// update bars
	wg := new(sync.WaitGroup)
	for _, bar := range []*pb.ProgressBar{first, second, third} {
		wg.Add(1)
		go func(cb *pb.ProgressBar) {
			for n := 0; n < 200; n++ {
				cb.Increment()
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(100)))
			}
			cb.Finish()
			wg.Done()
		}(bar)
	}
	wg.Wait()
	// close pool
	pool.Stop()
}
