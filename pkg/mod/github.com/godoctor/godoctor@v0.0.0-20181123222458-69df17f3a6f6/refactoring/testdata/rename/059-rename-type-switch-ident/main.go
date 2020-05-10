package main

import "fmt"


// Test for renaming the type switch variable
func main() {

var t interface{}
t = bool(true);           
switch y := t.(type) {  // <<<<< rename,11,8,11,8,renamed,pass            
default:
    fmt.Printf("unexpected type %T", y)  
case bool:
    fmt.Printf("boolean %t\n", y)            
case int:
    fmt.Printf("integer %d\n", y)            
case *bool:
    fmt.Printf("pointer to boolean %t\n", *y) 
case *int:
    fmt.Printf("pointer to integer %d\n", *y) 
}


 
}
