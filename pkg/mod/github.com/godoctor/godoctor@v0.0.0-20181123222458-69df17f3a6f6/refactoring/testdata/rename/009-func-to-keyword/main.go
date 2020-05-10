package main

import "fmt"


// Test for renaming the function  name to keyword

func main() {                           
                 
  	functionone()
	functiontwo()                            
                     
	
}

func functiontwo() {                        // <<<<< rename,16,6,16,6,func,fail                       

	var fun string = "this is a variable"            

		fmt.Println(fun)
                  
	fmt.Println("this is just for fun")                                                        

}

func functionone() {

fmt.Println("function one is called")

}
	
