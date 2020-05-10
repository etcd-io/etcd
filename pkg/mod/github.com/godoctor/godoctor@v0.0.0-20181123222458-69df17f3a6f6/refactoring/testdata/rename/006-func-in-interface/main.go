package main

import "fmt"

// Test for renaming the same function as method in interface

type simple interface {

fun() 

}


func main() {


fun()                  // <<<<< rename,17,1,17,1,renamed,pass


}

func fun() {


fmt.Println("fun is called")


}
