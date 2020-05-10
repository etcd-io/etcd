package pkg

type T1 struct {
	B int        `foo:"" foo:""` // want `duplicate struct tag`
	C int        `foo:"" bar:""`
	D int        `json:"-"`
	E int        `json:"\\"`                   // want `invalid JSON field name`
	F int        `json:",omitempty,omitempty"` // want `duplicate JSON option "omitempty"`
	G int        `json:",omitempty,string"`
	H int        `json:",string,omitempty,string"` // want `duplicate JSON option "string"`
	I int        `json:",unknown"`                 // want `unknown JSON option "unknown"`
	J int        `json:",string"`
	K *int       `json:",string"`
	L **int      `json:",string"` // want `the JSON string option`
	M complex128 `json:",string"` // want `the JSON string option`
	N int        `json:"some-name"`
	O int        `json:"some-name,inline"`
}

type T2 struct {
	A int `xml:",attr"`
	B int `xml:",chardata"`
	C int `xml:",cdata"`
	D int `xml:",innerxml"`
	E int `xml:",comment"`
	F int `xml:",omitempty"`
	G int `xml:",any"`
	H int `xml:",unknown"` // want `unknown XML option`
	I int `xml:",any,any"` // want `duplicate XML option`
	J int `xml:"a>b>c,"`
	K int `xml:",attr,cdata"` // want `mutually exclusive`
}

type T3 struct {
	A int `json:",omitempty" xml:",attr"`
	B int `json:",unknown" xml:",attr"` // want `unknown JSON option`
}
