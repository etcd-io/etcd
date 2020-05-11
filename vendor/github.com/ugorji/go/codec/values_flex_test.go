/* // +build testing */

// Copyright (c) 2012-2015 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

// This file contains values used by tests and benchmarks.

type TestStrucFlex struct {
	_struct struct{} `codec:",omitempty"` //set omitempty for every field
	testStrucCommon

	Mis     map[int]string
	Miwu64s map[int]wrapUint64Slice
	Mfwss   map[float64]wrapStringSlice
	Mf32wss map[float32]wrapStringSlice
	Mui2wss map[uint64]wrapStringSlice
	Msu2wss map[stringUint64T]wrapStringSlice

	//M map[interface{}]interface{}  `json:"-",bson:"-"`
	Mtsptr     map[string]*TestStrucFlex
	Mts        map[string]TestStrucFlex
	Its        []*TestStrucFlex
	Nteststruc *TestStrucFlex
}

func newTestStrucFlex(depth, n int, bench, useInterface, useStringKeyOnly bool) (ts *TestStrucFlex) {
	ts = &TestStrucFlex{
		Miwu64s: map[int]wrapUint64Slice{
			5: []wrapUint64{1, 2, 3, 4, 5},
			3: []wrapUint64{1, 2, 3},
		},

		Mf32wss: map[float32]wrapStringSlice{
			5.0: []wrapString{"1.0", "2.0", "3.0", "4.0", "5.0"},
			3.0: []wrapString{"1.0", "2.0", "3.0"},
		},

		Mui2wss: map[uint64]wrapStringSlice{
			5: []wrapString{"1.0", "2.0", "3.0", "4.0", "5.0"},
			3: []wrapString{"1.0", "2.0", "3.0"},
		},

		Mfwss: map[float64]wrapStringSlice{
			5.0: []wrapString{"1.0", "2.0", "3.0", "4.0", "5.0"},
			3.0: []wrapString{"1.0", "2.0", "3.0"},
		},
		Mis: map[int]string{
			1:   "one",
			22:  "twenty two",
			-44: "minus forty four",
		},
	}
	populateTestStrucCommon(&ts.testStrucCommon, n, bench, useInterface, useStringKeyOnly)
	if depth > 0 {
		depth--
		if ts.Mtsptr == nil {
			ts.Mtsptr = make(map[string]*TestStrucFlex)
		}
		if ts.Mts == nil {
			ts.Mts = make(map[string]TestStrucFlex)
		}
		ts.Mtsptr["0"] = newTestStrucFlex(depth, n, bench, useInterface, useStringKeyOnly)
		ts.Mts["0"] = *(ts.Mtsptr["0"])
		ts.Its = append(ts.Its, ts.Mtsptr["0"])
	}
	return
}
