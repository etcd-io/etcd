package main // <<<<< godoc,1,1,1,1,pass

import "fmt"

func main() {
	Exported()
}

func Exported() {
  fmt.Println("Hello, Go")
}

type Shaper interface {

}

type Rectangle struct {

}
