package pkg

import "encoding/json"

func fn1(i3 interface{}) {
	var v map[string]interface{}
	var i1 interface{} = v
	var i2 interface{} = &v
	p := &v
	json.Unmarshal([]byte(`{}`), v) // want `Unmarshal expects to unmarshal into a pointer`
	json.Unmarshal([]byte(`{}`), &v)
	json.Unmarshal([]byte(`{}`), i1) // want `Unmarshal expects to unmarshal into a pointer`
	json.Unmarshal([]byte(`{}`), i2)
	json.Unmarshal([]byte(`{}`), i3)
	json.Unmarshal([]byte(`{}`), p)

	json.NewDecoder(nil).Decode(v) // want `Decode expects to unmarshal into a pointer`
}
