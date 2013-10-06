package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// find the link local address for the given multicast address
func linkLocalAddr(maddr string) (laddr string, dev string, err error) {
	// get the interface device from the maddr
	parts := strings.Split(maddr, "%")
	if len(parts) == 1 {
		err = fmt.Errorf("invalid multicast address. must have a scope")
		return
	}
	dev = parts[1]
	// get the addr from the interface
	iface, err := net.InterfaceByName(dev)
	if err != nil {
		return
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return
	}
	for _, addr := range addrs {
		if addr.String()[:4] == "fe80" {
			laddr = strings.Split(addr.String(), "/")[0]
		}
	}
	if laddr == "" {
		err = fmt.Errorf("Could not find link local address for interface: %s", maddr)
	}
	return
}

// maddr is the multicast address group to attempt to find hosts at
func findNeighbours(maddr string) ([]net.Addr, error) {
	// find the link local address for this interface
	laddr, dev, err := linkLocalAddr(maddr)
	if err != nil {
		return nil, err
	}
	// listen on link local address only
	ra, err := net.ResolveIPAddr("ip6", laddr+"%"+dev)
	if err != nil {
		return nil, err
	}
	c, err := net.ListenIP("ip6:ipv6-icmp", ra)
	if err != nil {
		return nil, err
	}
	c.SetDeadline(time.Now().Add(1000 * time.Millisecond))
	defer c.Close()
	// set multicast group
	ip, err := net.ResolveIPAddr("ip6", maddr)
	if err != nil {
		return nil, err
	}
	// send icmp echo request
	id, seq := os.Getpid()&0xffff, 0
	b := make([]byte, 8)
	b[0] = 128
	b[4], b[5] = byte(id>>8), byte(id&0xff)
	b[6], b[7] = byte(seq>>8), byte(seq&0xff)
	if _, err := c.WriteTo(b, ip); err != nil {
		return nil, err
	}
	// collect as many neighbour addresses as possible before timeout
	neighbours := make([]net.Addr, 0)
	for {
		_, from, err := c.ReadFrom(b)
		if err != nil {
			switch e := err.(type) {
			case net.Error:
				if e.Timeout() {
					return neighbours, nil
				}
			}
			return nil, err
		}
		if b[0] == 129 && from.String() != laddr {
			neighbours = append(neighbours, from)
		}
	}
	return neighbours, nil
}
