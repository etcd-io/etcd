package pkg

func fn() {
	var ch chan int
	<-ch
	_ = <-ch // want `_ = <-ch`
	select {
	case <-ch:
	case _ = <-ch: // want `_ = <-ch`
	}
	x := <-ch
	y, _ := <-ch, <-ch // want `_ = <-ch`
	_, z := <-ch, <-ch // want `_ = <-ch`
	_, _, _ = x, y, z
}
