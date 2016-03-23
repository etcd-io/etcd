package pcap

import (
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"strings"
)

const (
	TYPE_IP   = 0x0800
	TYPE_ARP  = 0x0806
	TYPE_IP6  = 0x86DD
	TYPE_VLAN = 0x8100

	IP_ICMP = 1
	IP_INIP = 4
	IP_TCP  = 6
	IP_UDP  = 17
)

const (
	ERRBUF_SIZE = 256

	// According to pcap-linktype(7).
	LINKTYPE_NULL             = 0
	LINKTYPE_ETHERNET         = 1
	LINKTYPE_TOKEN_RING       = 6
	LINKTYPE_ARCNET           = 7
	LINKTYPE_SLIP             = 8
	LINKTYPE_PPP              = 9
	LINKTYPE_FDDI             = 10
	LINKTYPE_ATM_RFC1483      = 100
	LINKTYPE_RAW              = 101
	LINKTYPE_PPP_HDLC         = 50
	LINKTYPE_PPP_ETHER        = 51
	LINKTYPE_C_HDLC           = 104
	LINKTYPE_IEEE802_11       = 105
	LINKTYPE_FRELAY           = 107
	LINKTYPE_LOOP             = 108
	LINKTYPE_LINUX_SLL        = 113
	LINKTYPE_LTALK            = 104
	LINKTYPE_PFLOG            = 117
	LINKTYPE_PRISM_HEADER     = 119
	LINKTYPE_IP_OVER_FC       = 122
	LINKTYPE_SUNATM           = 123
	LINKTYPE_IEEE802_11_RADIO = 127
	LINKTYPE_ARCNET_LINUX     = 129
	LINKTYPE_LINUX_IRDA       = 144
	LINKTYPE_LINUX_LAPD       = 177
)

type addrHdr interface {
	SrcAddr() string
	DestAddr() string
	Len() int
}

type addrStringer interface {
	String(addr addrHdr) string
}

func decodemac(pkt []byte) uint64 {
	mac := uint64(0)
	for i := uint(0); i < 6; i++ {
		mac = (mac << 8) + uint64(pkt[i])
	}
	return mac
}

// Decode decodes the headers of a Packet.
func (p *Packet) Decode() {
	if len(p.Data) <= 14 {
		return
	}

	p.Type = int(binary.BigEndian.Uint16(p.Data[12:14]))
	p.DestMac = decodemac(p.Data[0:6])
	p.SrcMac = decodemac(p.Data[6:12])

	if len(p.Data) >= 15 {
		p.Payload = p.Data[14:]
	}

	switch p.Type {
	case TYPE_IP:
		p.decodeIp()
	case TYPE_IP6:
		p.decodeIp6()
	case TYPE_ARP:
		p.decodeArp()
	case TYPE_VLAN:
		p.decodeVlan()
	}
}

func (p *Packet) headerString(headers []interface{}) string {
	// If there's just one header, return that.
	if len(headers) == 1 {
		if hdr, ok := headers[0].(fmt.Stringer); ok {
			return hdr.String()
		}
	}
	// If there are two headers (IPv4/IPv6 -> TCP/UDP/IP..)
	if len(headers) == 2 {
		// Commonly the first header is an address.
		if addr, ok := p.Headers[0].(addrHdr); ok {
			if hdr, ok := p.Headers[1].(addrStringer); ok {
				return fmt.Sprintf("%s %s", p.Time, hdr.String(addr))
			}
		}
	}
	// For IP in IP, we do a recursive call.
	if len(headers) >= 2 {
		if addr, ok := headers[0].(addrHdr); ok {
			if _, ok := headers[1].(addrHdr); ok {
				return fmt.Sprintf("%s > %s IP in IP: ",
					addr.SrcAddr(), addr.DestAddr(), p.headerString(headers[1:]))
			}
		}
	}

	var typeNames []string
	for _, hdr := range headers {
		typeNames = append(typeNames, reflect.TypeOf(hdr).String())
	}

	return fmt.Sprintf("unknown [%s]", strings.Join(typeNames, ","))
}

// String prints a one-line representation of the packet header.
// The output is suitable for use in a tcpdump program.
func (p *Packet) String() string {
	// If there are no headers, print "unsupported protocol".
	if len(p.Headers) == 0 {
		return fmt.Sprintf("%s unsupported protocol %d", p.Time, int(p.Type))
	}
	return fmt.Sprintf("%s %s", p.Time, p.headerString(p.Headers))
}

// Arphdr is a ARP packet header.
type Arphdr struct {
	Addrtype          uint16
	Protocol          uint16
	HwAddressSize     uint8
	ProtAddressSize   uint8
	Operation         uint16
	SourceHwAddress   []byte
	SourceProtAddress []byte
	DestHwAddress     []byte
	DestProtAddress   []byte
}

func (arp *Arphdr) String() (s string) {
	switch arp.Operation {
	case 1:
		s = "ARP request"
	case 2:
		s = "ARP Reply"
	}
	if arp.Addrtype == LINKTYPE_ETHERNET && arp.Protocol == TYPE_IP {
		s = fmt.Sprintf("%012x (%s) > %012x (%s)",
			decodemac(arp.SourceHwAddress), arp.SourceProtAddress,
			decodemac(arp.DestHwAddress), arp.DestProtAddress)
	} else {
		s = fmt.Sprintf("addrtype = %d protocol = %d", arp.Addrtype, arp.Protocol)
	}
	return
}

func (p *Packet) decodeArp() {
	if len(p.Payload) < 8 {
		return
	}

	pkt := p.Payload
	arp := new(Arphdr)
	arp.Addrtype = binary.BigEndian.Uint16(pkt[0:2])
	arp.Protocol = binary.BigEndian.Uint16(pkt[2:4])
	arp.HwAddressSize = pkt[4]
	arp.ProtAddressSize = pkt[5]
	arp.Operation = binary.BigEndian.Uint16(pkt[6:8])

	if len(pkt) < int(8+2*arp.HwAddressSize+2*arp.ProtAddressSize) {
		return
	}
	arp.SourceHwAddress = pkt[8 : 8+arp.HwAddressSize]
	arp.SourceProtAddress = pkt[8+arp.HwAddressSize : 8+arp.HwAddressSize+arp.ProtAddressSize]
	arp.DestHwAddress = pkt[8+arp.HwAddressSize+arp.ProtAddressSize : 8+2*arp.HwAddressSize+arp.ProtAddressSize]
	arp.DestProtAddress = pkt[8+2*arp.HwAddressSize+arp.ProtAddressSize : 8+2*arp.HwAddressSize+2*arp.ProtAddressSize]

	p.Headers = append(p.Headers, arp)

	if len(pkt) >= int(8+2*arp.HwAddressSize+2*arp.ProtAddressSize) {
		p.Payload = p.Payload[8+2*arp.HwAddressSize+2*arp.ProtAddressSize:]
	}
}

// IPadr is the header of an IP packet.
type Iphdr struct {
	Version    uint8
	Ihl        uint8
	Tos        uint8
	Length     uint16
	Id         uint16
	Flags      uint8
	FragOffset uint16
	Ttl        uint8
	Protocol   uint8
	Checksum   uint16
	SrcIp      []byte
	DestIp     []byte
}

func (p *Packet) decodeIp() {
	if len(p.Payload) < 20 {
		return
	}

	pkt := p.Payload
	ip := new(Iphdr)

	ip.Version = uint8(pkt[0]) >> 4
	ip.Ihl = uint8(pkt[0]) & 0x0F
	ip.Tos = pkt[1]
	ip.Length = binary.BigEndian.Uint16(pkt[2:4])
	ip.Id = binary.BigEndian.Uint16(pkt[4:6])
	flagsfrags := binary.BigEndian.Uint16(pkt[6:8])
	ip.Flags = uint8(flagsfrags >> 13)
	ip.FragOffset = flagsfrags & 0x1FFF
	ip.Ttl = pkt[8]
	ip.Protocol = pkt[9]
	ip.Checksum = binary.BigEndian.Uint16(pkt[10:12])
	ip.SrcIp = pkt[12:16]
	ip.DestIp = pkt[16:20]

	pEnd := int(ip.Length)
	if pEnd > len(pkt) {
		pEnd = len(pkt)
	}

	if len(pkt) >= pEnd && int(ip.Ihl*4) < pEnd {
		p.Payload = pkt[ip.Ihl*4 : pEnd]
	} else {
		p.Payload = []byte{}
	}

	p.Headers = append(p.Headers, ip)
	p.IP = ip

	switch ip.Protocol {
	case IP_TCP:
		p.decodeTcp()
	case IP_UDP:
		p.decodeUdp()
	case IP_ICMP:
		p.decodeIcmp()
	case IP_INIP:
		p.decodeIp()
	}
}

func (ip *Iphdr) SrcAddr() string  { return net.IP(ip.SrcIp).String() }
func (ip *Iphdr) DestAddr() string { return net.IP(ip.DestIp).String() }
func (ip *Iphdr) Len() int         { return int(ip.Length) }

type Vlanhdr struct {
	Priority       byte
	DropEligible   bool
	VlanIdentifier int
	Type           int // Not actually part of the vlan header, but the type of the actual packet
}

func (v *Vlanhdr) String() {
	fmt.Sprintf("VLAN Priority:%d Drop:%v Tag:%d", v.Priority, v.DropEligible, v.VlanIdentifier)
}

func (p *Packet) decodeVlan() {
	pkt := p.Payload
	vlan := new(Vlanhdr)
	if len(pkt) < 4 {
		return
	}

	vlan.Priority = (pkt[2] & 0xE0) >> 13
	vlan.DropEligible = pkt[2]&0x10 != 0
	vlan.VlanIdentifier = int(binary.BigEndian.Uint16(pkt[:2])) & 0x0FFF
	vlan.Type = int(binary.BigEndian.Uint16(p.Payload[2:4]))
	p.Headers = append(p.Headers, vlan)

	if len(pkt) >= 5 {
		p.Payload = p.Payload[4:]
	}

	switch vlan.Type {
	case TYPE_IP:
		p.decodeIp()
	case TYPE_IP6:
		p.decodeIp6()
	case TYPE_ARP:
		p.decodeArp()
	}
}

type Tcphdr struct {
	SrcPort    uint16
	DestPort   uint16
	Seq        uint32
	Ack        uint32
	DataOffset uint8
	Flags      uint16
	Window     uint16
	Checksum   uint16
	Urgent     uint16
	Data       []byte
}

const (
	TCP_FIN = 1 << iota
	TCP_SYN
	TCP_RST
	TCP_PSH
	TCP_ACK
	TCP_URG
	TCP_ECE
	TCP_CWR
	TCP_NS
)

func (p *Packet) decodeTcp() {
	if len(p.Payload) < 20 {
		return
	}

	pkt := p.Payload
	tcp := new(Tcphdr)
	tcp.SrcPort = binary.BigEndian.Uint16(pkt[0:2])
	tcp.DestPort = binary.BigEndian.Uint16(pkt[2:4])
	tcp.Seq = binary.BigEndian.Uint32(pkt[4:8])
	tcp.Ack = binary.BigEndian.Uint32(pkt[8:12])
	tcp.DataOffset = (pkt[12] & 0xF0) >> 4
	tcp.Flags = binary.BigEndian.Uint16(pkt[12:14]) & 0x1FF
	tcp.Window = binary.BigEndian.Uint16(pkt[14:16])
	tcp.Checksum = binary.BigEndian.Uint16(pkt[16:18])
	tcp.Urgent = binary.BigEndian.Uint16(pkt[18:20])
	if len(pkt) >= int(tcp.DataOffset*4) {
		p.Payload = pkt[tcp.DataOffset*4:]
	}
	p.Headers = append(p.Headers, tcp)
	p.TCP = tcp
}

func (tcp *Tcphdr) String(hdr addrHdr) string {
	return fmt.Sprintf("TCP %s:%d > %s:%d %s SEQ=%d ACK=%d LEN=%d",
		hdr.SrcAddr(), int(tcp.SrcPort), hdr.DestAddr(), int(tcp.DestPort),
		tcp.FlagsString(), int64(tcp.Seq), int64(tcp.Ack), hdr.Len())
}

func (tcp *Tcphdr) FlagsString() string {
	var sflags []string
	if 0 != (tcp.Flags & TCP_SYN) {
		sflags = append(sflags, "syn")
	}
	if 0 != (tcp.Flags & TCP_FIN) {
		sflags = append(sflags, "fin")
	}
	if 0 != (tcp.Flags & TCP_ACK) {
		sflags = append(sflags, "ack")
	}
	if 0 != (tcp.Flags & TCP_PSH) {
		sflags = append(sflags, "psh")
	}
	if 0 != (tcp.Flags & TCP_RST) {
		sflags = append(sflags, "rst")
	}
	if 0 != (tcp.Flags & TCP_URG) {
		sflags = append(sflags, "urg")
	}
	if 0 != (tcp.Flags & TCP_NS) {
		sflags = append(sflags, "ns")
	}
	if 0 != (tcp.Flags & TCP_CWR) {
		sflags = append(sflags, "cwr")
	}
	if 0 != (tcp.Flags & TCP_ECE) {
		sflags = append(sflags, "ece")
	}
	return fmt.Sprintf("[%s]", strings.Join(sflags, " "))
}

type Udphdr struct {
	SrcPort  uint16
	DestPort uint16
	Length   uint16
	Checksum uint16
}

func (p *Packet) decodeUdp() {
	if len(p.Payload) < 8 {
		return
	}

	pkt := p.Payload
	udp := new(Udphdr)
	udp.SrcPort = binary.BigEndian.Uint16(pkt[0:2])
	udp.DestPort = binary.BigEndian.Uint16(pkt[2:4])
	udp.Length = binary.BigEndian.Uint16(pkt[4:6])
	udp.Checksum = binary.BigEndian.Uint16(pkt[6:8])
	p.Headers = append(p.Headers, udp)
	p.UDP = udp
	if len(p.Payload) >= 8 {
		p.Payload = pkt[8:]
	}
}

func (udp *Udphdr) String(hdr addrHdr) string {
	return fmt.Sprintf("UDP %s:%d > %s:%d LEN=%d CHKSUM=%d",
		hdr.SrcAddr(), int(udp.SrcPort), hdr.DestAddr(), int(udp.DestPort),
		int(udp.Length), int(udp.Checksum))
}

type Icmphdr struct {
	Type     uint8
	Code     uint8
	Checksum uint16
	Id       uint16
	Seq      uint16
	Data     []byte
}

func (p *Packet) decodeIcmp() *Icmphdr {
	if len(p.Payload) < 8 {
		return nil
	}

	pkt := p.Payload
	icmp := new(Icmphdr)
	icmp.Type = pkt[0]
	icmp.Code = pkt[1]
	icmp.Checksum = binary.BigEndian.Uint16(pkt[2:4])
	icmp.Id = binary.BigEndian.Uint16(pkt[4:6])
	icmp.Seq = binary.BigEndian.Uint16(pkt[6:8])
	p.Payload = pkt[8:]
	p.Headers = append(p.Headers, icmp)
	return icmp
}

func (icmp *Icmphdr) String(hdr addrHdr) string {
	return fmt.Sprintf("ICMP %s > %s Type = %d Code = %d ",
		hdr.SrcAddr(), hdr.DestAddr(), icmp.Type, icmp.Code)
}

func (icmp *Icmphdr) TypeString() (result string) {
	switch icmp.Type {
	case 0:
		result = fmt.Sprintf("Echo reply seq=%d", icmp.Seq)
	case 3:
		switch icmp.Code {
		case 0:
			result = "Network unreachable"
		case 1:
			result = "Host unreachable"
		case 2:
			result = "Protocol unreachable"
		case 3:
			result = "Port unreachable"
		default:
			result = "Destination unreachable"
		}
	case 8:
		result = fmt.Sprintf("Echo request seq=%d", icmp.Seq)
	case 30:
		result = "Traceroute"
	}
	return
}

type Ip6hdr struct {
	// http://www.networksorcery.com/enp/protocol/ipv6.htm
	Version      uint8  // 4 bits
	TrafficClass uint8  // 8 bits
	FlowLabel    uint32 // 20 bits
	Length       uint16 // 16 bits
	NextHeader   uint8  // 8 bits, same as Protocol in Iphdr
	HopLimit     uint8  // 8 bits
	SrcIp        []byte // 16 bytes
	DestIp       []byte // 16 bytes
}

func (p *Packet) decodeIp6() {
	if len(p.Payload) < 40 {
		return
	}

	pkt := p.Payload
	ip6 := new(Ip6hdr)
	ip6.Version = uint8(pkt[0]) >> 4
	ip6.TrafficClass = uint8((binary.BigEndian.Uint16(pkt[0:2]) >> 4) & 0x00FF)
	ip6.FlowLabel = binary.BigEndian.Uint32(pkt[0:4]) & 0x000FFFFF
	ip6.Length = binary.BigEndian.Uint16(pkt[4:6])
	ip6.NextHeader = pkt[6]
	ip6.HopLimit = pkt[7]
	ip6.SrcIp = pkt[8:24]
	ip6.DestIp = pkt[24:40]

	if len(p.Payload) >= 40 {
		p.Payload = pkt[40:]
	}

	p.Headers = append(p.Headers, ip6)

	switch ip6.NextHeader {
	case IP_TCP:
		p.decodeTcp()
	case IP_UDP:
		p.decodeUdp()
	case IP_ICMP:
		p.decodeIcmp()
	case IP_INIP:
		p.decodeIp()
	}
}

func (ip6 *Ip6hdr) SrcAddr() string  { return net.IP(ip6.SrcIp).String() }
func (ip6 *Ip6hdr) DestAddr() string { return net.IP(ip6.DestIp).String() }
func (ip6 *Ip6hdr) Len() int         { return int(ip6.Length) }
