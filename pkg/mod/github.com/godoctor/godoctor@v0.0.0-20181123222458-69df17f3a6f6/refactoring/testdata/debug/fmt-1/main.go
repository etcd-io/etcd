package main
import "fmt"
type t int
func  (s  *  t) String ( ) string  { return String( ) } //<<<<<debug,1,1,1,11,fmt,pass
func String ( )string { return "" } ; func main() { var i t = 3; fmt.Println(i.String()) }
