package secondpackage 

import (
"mypackage"
)

func SecondFunction() int {             
	
    x := mypackage.MyFunction(10)
    
    x = x*100
  
   return x 
}

func IsEven(n int) bool {

	if n%2==0 {
  
		return true 
   	}

	return false

}
