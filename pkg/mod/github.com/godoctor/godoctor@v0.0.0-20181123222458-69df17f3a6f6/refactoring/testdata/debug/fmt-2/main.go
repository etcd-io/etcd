package main
import "fmt"
type t int
func  (s  *  t) String ( ) /* */ string  { return String( ) } //<<<<<debug,4,1,4,1,fmt,pass
func String ( )string { return "" } ; func main() { var i t = 3; fmt.Println(i.String()) }
