package cli

import (
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

var (
	wd, _ = os.Getwd()
)

func expect(t *testing.T, a interface{}, b interface{}) {
	_, fn, line, _ := runtime.Caller(1)
	fn = strings.Replace(fn, wd+"/", "", -1)

	if !reflect.DeepEqual(a, b) {
		t.Errorf("(%s:%d) Expected %v (type %v) - Got %v (type %v)", fn, line, b, reflect.TypeOf(b), a, reflect.TypeOf(a))
	}
}

func refute(t *testing.T, a interface{}, b interface{}) {
	if reflect.DeepEqual(a, b) {
		t.Errorf("Did not expect %v (type %v) - Got %v (type %v)", b, reflect.TypeOf(b), a, reflect.TypeOf(a))
	}
}
