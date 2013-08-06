package raft

import (
	"testing"
	"time"
)

//------------------------------------------------------------------------------
//
// Tests
//
//------------------------------------------------------------------------------

// Ensure that we can start an election timer and it will go off in the specified duration.
func TestTimer(t *testing.T) {
	timer := newTimer(5*time.Millisecond, 10*time.Millisecond)

	// test timer start
	for i := 0; i < 10; i++ {
		start := time.Now()
		timer.start()

		duration := time.Now().Sub(start)
		if duration > 12*time.Millisecond || duration < 5*time.Millisecond {
			t.Fatal("Duration Error! ", duration)
		}
	}

	// test timer stop
	for i := 0; i < 100; i++ {
		start := time.Now()
		go stop(timer)
		timer.start()

		duration := time.Now().Sub(start)
		if duration > 3*time.Millisecond {
			t.Fatal("Duration Error! ", duration)
		}

		// ready the timer after stop it
		timer.ready()
	}

	// test timer fire
	for i := 0; i < 100; i++ {
		start := time.Now()
		go fire(timer)
		timer.start()

		duration := time.Now().Sub(start)
		if duration > 3*time.Millisecond {
			t.Fatal("Fire Duration Error! ", duration)
		}
	}

	resp := make(chan bool)

	// play with start and stop
	// make sure we can stop timer
	// in all the possible seq of start and stop
	for i := 0; i < 100; i++ {
		go stop(timer)
		go start(timer, resp)
		ret := <-resp
		if ret != false {
			t.Fatal("cannot stop timer!")
		}
		timer.ready()
	}

}

func stop(t *timer) {
	time.Sleep(time.Millisecond)
	t.stop()
}

func start(t *timer, resp chan bool) {
	time.Sleep(time.Millisecond)
	resp <- t.start()
}

func fire(t *timer) {
	time.Sleep(time.Millisecond)
	t.fire()
}
