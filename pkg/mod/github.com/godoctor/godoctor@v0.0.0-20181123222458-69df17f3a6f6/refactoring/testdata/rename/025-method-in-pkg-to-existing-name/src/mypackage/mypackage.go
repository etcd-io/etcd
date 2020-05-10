package mypackage 


type Mystruct struct {

Myvar string

}

func (mystructvar *Mystruct)Mymethod() string {


return mystructvar.Myvar


}

func (mystructvar *Mystruct)Secondmethod() string {


return mystructvar.Myvar+"helloo"


}

