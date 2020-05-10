package mypackage 



type Simple interface {

Mymethod() string

} 


type Mystruct struct {

Myvar string

}

func (mystructvar *Mystruct)Mymethod() string {


return mystructvar.Myvar


}


