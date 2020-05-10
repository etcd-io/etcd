package mypackage 


type Sample interface {

Mymethod() string

}


type Mystruct struct {

Myvar string

}

type Otherstruct struct {

Othervar string 

}



func (mystructvar Mystruct)Mymethod() string {


return mystructvar.Myvar


}

func (otherstructvar Otherstruct)Mymethod() string {


return otherstructvar.Othervar


}

