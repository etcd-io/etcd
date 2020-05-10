package mypackage 



type Simple interface {

Mymethod() string

} 

type Complex interface {

Mymethod() int

}

type Third interface {

Renamed() string
ThirdOnly()

}

type Mystruct struct {

Myvar string

}

func (mystructvar *Mystruct)Mymethod() string {


return mystructvar.Myvar


}

func (mystructvar *Mystruct) ThirdOnly() {
}
