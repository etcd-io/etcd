package secondpackage 

import (
"fmt"
"mypackage"
)

type Secondstruct struct {

Secondvar string

}

func SecondFunction() {             
	
mystructvar := mypackage.Mystruct {"Hiii" }  

fmt.Println("value is",mystructvar.Mymethod())	

}



func (secondstructvar *Secondstruct)Secondmethod() string {


return secondstructvar.Secondvar


}

