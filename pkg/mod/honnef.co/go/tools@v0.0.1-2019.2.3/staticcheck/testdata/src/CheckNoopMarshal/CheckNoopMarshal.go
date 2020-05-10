package pkg

import (
	"encoding/json"
	"encoding/xml"
)

type T1 struct{}
type T2 struct{ x int }
type T3 struct{ X int }
type T4 struct{ T3 }
type t5 struct{ X int }
type T6 struct{ t5 }
type T7 struct{ x int }

func (T7) MarshalJSON() ([]byte, error) { return nil, nil }
func (*T7) UnmarshalJSON([]byte) error  { return nil }

type T8 struct{ x int }

func (T8) MarshalXML() ([]byte, error)                         { return nil, nil }
func (*T8) UnmarshalXML(*xml.Decoder, *xml.StartElement) error { return nil }

type T9 struct{}

func (T9) MarshalText() ([]byte, error) { return nil, nil }
func (*T9) UnmarshalText([]byte) error  { return nil }

type T10 struct{}
type T11 struct{ T10 }
type T12 struct{ T7 }
type t13 struct{}

func (t13) MarshalJSON() ([]byte, error) { return nil, nil }

type T14 struct{ t13 }
type T15 struct{ *t13 }
type T16 struct{ *T3 }
type T17 struct{ *T17 }
type T18 struct {
	T17
	Actual int
}

func fn() {
	// don't flag structs with no fields
	json.Marshal(T1{})
	// no exported fields
	json.Marshal(T2{}) // want `struct doesn't have any exported fields, nor custom marshaling`
	// pointer vs non-pointer makes no difference
	json.Marshal(&T2{}) // want `struct doesn't have any exported fields, nor custom marshaling`
	// exported field
	json.Marshal(T3{})
	// exported field, pointer makes no difference
	json.Marshal(&T3{})
	// embeds struct with exported fields
	json.Marshal(T4{})
	// exported field
	json.Marshal(t5{})
	// embeds unexported type, but said type does have exported fields
	json.Marshal(T6{})
	// MarshalJSON
	json.Marshal(T7{})
	// MarshalXML does not apply to JSON
	json.Marshal(T8{}) // want `struct doesn't have any exported fields, nor custom marshaling`
	// MarshalText
	json.Marshal(T9{})
	// embeds exported struct, but it has no fields
	json.Marshal(T11{}) // want `struct doesn't have any exported fields, nor custom marshaling`
	// embeds type with MarshalJSON
	json.Marshal(T12{})
	// embeds type with MarshalJSON and type isn't exported
	json.Marshal(T14{})
	// embedded pointer with MarshalJSON
	json.Marshal(T15{})
	// embedded pointer to struct with exported fields
	json.Marshal(T16{})
	// don't recurse forever on recursive data structure
	json.Marshal(T17{}) // want `struct doesn't have any exported fields, nor custom marshaling`
	json.Marshal(T18{})

	// MarshalJSON does not apply to JSON
	xml.Marshal(T7{}) // want `struct doesn't have any exported fields, nor custom marshaling`
	// MarshalXML
	xml.Marshal(T8{})

	var t2 T2
	var t3 T3
	var t7 T7
	var t8 T8
	var t9 T9
	// check that all other variations of methods also work
	json.Unmarshal(nil, &t2) // want `struct doesn't have any exported fields, nor custom marshaling`
	json.Unmarshal(nil, &t3)
	json.Unmarshal(nil, &t9)
	xml.Unmarshal(nil, &t2) // want `struct doesn't have any exported fields, nor custom marshaling`
	xml.Unmarshal(nil, &t3)
	xml.Unmarshal(nil, &t9)
	(*json.Decoder)(nil).Decode(&t2) // want `struct doesn't have any exported fields, nor custom marshaling`
	(*json.Decoder)(nil).Decode(&t3)
	(*json.Decoder)(nil).Decode(&t9)
	(*json.Encoder)(nil).Encode(t2) // want `struct doesn't have any exported fields, nor custom marshaling`
	(*json.Encoder)(nil).Encode(t3)
	(*json.Encoder)(nil).Encode(t9)
	(*xml.Decoder)(nil).Decode(&t2) // want `struct doesn't have any exported fields, nor custom marshaling`
	(*xml.Decoder)(nil).Decode(&t3)
	(*xml.Decoder)(nil).Decode(&t9)
	(*xml.Encoder)(nil).Encode(t2) // want `struct doesn't have any exported fields, nor custom marshaling`
	(*xml.Encoder)(nil).Encode(t3)
	(*xml.Encoder)(nil).Encode(t9)

	(*json.Decoder)(nil).Decode(&t7)
	(*json.Decoder)(nil).Decode(&t8) // want `struct doesn't have any exported fields, nor custom marshaling`
	(*json.Encoder)(nil).Encode(t7)
	(*json.Encoder)(nil).Encode(t8) // want `struct doesn't have any exported fields, nor custom marshaling`
	(*xml.Decoder)(nil).Decode(&t7) // want `struct doesn't have any exported fields, nor custom marshaling`
	(*xml.Decoder)(nil).Decode(&t8)
	(*xml.Encoder)(nil).Encode(t7) // want `struct doesn't have any exported fields, nor custom marshaling`
	(*xml.Encoder)(nil).Encode(t8)

}

var _, _ = json.Marshal(T9{})
