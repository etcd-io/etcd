package pkg

/*
#include <stdlib.h>

int myVar = 42;

// return a global variable
int mulMyVar(int n) {
  return myVar * n;
}


*/
import "C"
import "fmt"

func DoPackageStuff() {
	fmt.Println(C.mulMyVar(2))
	fmt.Println(C.rand())
}
