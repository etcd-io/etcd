package pkg

import "reflect"

type wkt interface {
	XXX_WellKnownType() string
}

var typeOfWkt = reflect.TypeOf((*wkt)(nil)).Elem()

func Fn() {
	_ = typeOfWkt
}

type t *int

var _ t
