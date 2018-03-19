package balancer

import (
	"reflect"
	"testing"

	"google.golang.org/grpc/resolver"
)

func Test_epsToAddrs(t *testing.T) {
	eps := []string{"https://example.com:2379", "127.0.0.1:2379"}
	exp := []resolver.Address{
		{Addr: "example.com:2379", Type: resolver.Backend},
		{Addr: "127.0.0.1:2379", Type: resolver.Backend},
	}
	rs := epsToAddrs(eps...)
	if !reflect.DeepEqual(rs, exp) {
		t.Fatalf("expected %v, got %v", exp, rs)
	}
}
