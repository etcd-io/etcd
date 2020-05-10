package pkg

import (
	"encoding/json"
	"encoding/xml"
)

type T1 struct {
	A int
	B func() `json:"-" xml:"-"`
	c chan int
}

type T2 struct {
	T1
}

type T3 struct {
	C chan int
}

type T4 struct {
	C C
}

type T5 struct {
	B func() `xml:"-"`
}

type T6 struct {
	B func() `json:"-"`
}

type T7 struct {
	A int
	B int
	T3
}

type T8 struct {
	C int
	*T7
}

type C chan int

func (C) MarshalText() ([]byte, error) { return nil, nil }

func fn() {
	var t1 T1
	var t2 T2
	var t3 T3
	var t4 T4
	var t5 T5
	var t6 T6
	var t8 T8
	json.Marshal(t1)
	json.Marshal(t2)
	json.Marshal(t3) // want `trying to marshal chan or func value, field CheckUnsupportedMarshal\.T3\.C`
	json.Marshal(t4)
	json.Marshal(t5) // want `trying to marshal chan or func value, field CheckUnsupportedMarshal\.T5\.B`
	json.Marshal(t6)
	(*json.Encoder)(nil).Encode(t1)
	(*json.Encoder)(nil).Encode(t2)
	(*json.Encoder)(nil).Encode(t3) // want `trying to marshal chan or func value, field CheckUnsupportedMarshal\.T3\.C`
	(*json.Encoder)(nil).Encode(t4)
	(*json.Encoder)(nil).Encode(t5) // want `trying to marshal chan or func value, field CheckUnsupportedMarshal\.T5\.B`
	(*json.Encoder)(nil).Encode(t6)

	xml.Marshal(t1)
	xml.Marshal(t2)
	xml.Marshal(t3) // want `trying to marshal chan or func value, field CheckUnsupportedMarshal\.T3\.C`
	xml.Marshal(t4)
	xml.Marshal(t5)
	xml.Marshal(t6) // want `trying to marshal chan or func value, field CheckUnsupportedMarshal\.T6\.B`
	(*xml.Encoder)(nil).Encode(t1)
	(*xml.Encoder)(nil).Encode(t2)
	(*xml.Encoder)(nil).Encode(t3) // want `trying to marshal chan or func value, field CheckUnsupportedMarshal\.T3\.C`
	(*xml.Encoder)(nil).Encode(t4)
	(*xml.Encoder)(nil).Encode(t5)
	(*xml.Encoder)(nil).Encode(t6) // want `trying to marshal chan or func value, field CheckUnsupportedMarshal\.T6\.B`

	json.Marshal(t8) // want `trying to marshal chan or func value, field CheckUnsupportedMarshal\.T8\.T7\.T3\.C`
}
