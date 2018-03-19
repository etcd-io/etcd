package balancer

import (
	"fmt"
	"net/url"
	"sort"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/resolver"
)

func scToString(sc balancer.SubConn) string {
	return fmt.Sprintf("%p", sc)
}

func scsToStrings(scs map[resolver.Address]balancer.SubConn) (ss []string) {
	ss = make([]string, 0, len(scs))
	for a, sc := range scs {
		ss = append(ss, fmt.Sprintf("%s (%s)", a.Addr, scToString(sc)))
	}
	sort.Strings(ss)
	return ss
}

func addrsToStrings(addrs []resolver.Address) (ss []string) {
	ss = make([]string, len(addrs))
	for i := range addrs {
		ss[i] = addrs[i].Addr
	}
	sort.Strings(ss)
	return ss
}

func epsToAddrs(eps ...string) (addrs []resolver.Address) {
	addrs = make([]resolver.Address, 0, len(eps))
	for _, ep := range eps {
		u, err := url.Parse(ep)
		if err != nil {
			addrs = append(addrs, resolver.Address{Addr: ep, Type: resolver.Backend})
			continue
		}
		addrs = append(addrs, resolver.Address{Addr: u.Host, Type: resolver.Backend})
	}
	return addrs
}
