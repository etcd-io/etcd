// Package fields provides types and functions to work with key-value pairs.
//
// The package offers three primary abstractions:
//
//   - Field: A key-value pair where the key is a string and the value can be any type.
//   - Dict: A map-based collection of unique fields, providing efficient key-based lookup.
//   - List: An ordered collection of fields that preserves insertion order.
//
// Fields can be created using the F constructor, and both Dict and List provide
// conversion methods between the two collection types. All types implement String()
// for consistent string representation.
//
// Example usage:
//
//	// Create fields
//	f1 := fields.F("status", "success")
//	f2 := fields.F("code", 200)
//
//	// Working with a List (ordered collection)
//	var list fields.List
//	list.Add(f1, f2)
//	fmt.Println(list) // "(status=success, code=200)"
//
//	// Working with a Dict (unique key collection)
//	dict := fields.Dict{}
//	dict.Add(f1, f2, fields.F("status", "updated")) // overwrites "status"
//	fmt.Println(dict) // "(status=updated, code=200)" (order may vary)
//
//	// Converting between types
//	list2 := dict.ToList() // order unspecified
//	dict2 := list.ToDict() // last occurrence of each key wins
package fields
