package main

/*
#include <stdlib.h>

int myVar = 42;

// simple square calculation
int sqr(int a) {
  return a * a;
}

// return a global variable
int returnMyVar() {
  return myVar;
}


*/
import "C"
import "fmt"

// NOTE: cgo doesn't work properly if import ( ... ) is used instead

func main() {
	fmt.Println(C.sqr(2)) // <<<<< rename,26,16,26,18,foo,fail
	fmt.Println(C.returnMyVar())
	fmt.Println(C.myVar)
	fmt.Println(C.rand())
}
