// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: v3lock.proto

/*
	Package v3lockpb is a generated protocol buffer package.

	It is generated from these files:
		v3lock.proto

	It has these top-level messages:
		LockRequest
		LockResponse
		UnlockRequest
		UnlockResponse
*/
package v3lockpb

import (
	"fmt"

	proto "github.com/golang/protobuf/proto"

	math "math"

	_ "github.com/gogo/protobuf/gogoproto"

	etcdserverpb "go.etcd.io/etcd/etcdserver/etcdserverpb"

	context "golang.org/x/net/context"

	grpc "google.golang.org/grpc"

	io "io"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type LockRequest struct {
	// name is the identifier for the distributed shared lock to be acquired.
	Name []byte `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// lease is the ID of the lease that will be attached to ownership of the
	// lock. If the lease expires or is revoked and currently holds the lock,
	// the lock is automatically released. Calls to Lock with the same lease will
	// be treated as a single acquisition; locking twice with the same lease is a
	// no-op.
	Lease int64 `protobuf:"varint,2,opt,name=lease,proto3" json:"lease,omitempty"`
}

func (m *LockRequest) Reset()                    { *m = LockRequest{} }
func (m *LockRequest) String() string            { return proto.CompactTextString(m) }
func (*LockRequest) ProtoMessage()               {}
func (*LockRequest) Descriptor() ([]byte, []int) { return fileDescriptorV3Lock, []int{0} }

func (m *LockRequest) GetName() []byte {
	if m != nil {
		return m.Name
	}
	return nil
}

func (m *LockRequest) GetLease() int64 {
	if m != nil {
		return m.Lease
	}
	return 0
}

type LockResponse struct {
	Header *etcdserverpb.ResponseHeader `protobuf:"bytes,1,opt,name=header" json:"header,omitempty"`
	// key is a key that will exist on etcd for the duration that the Lock caller
	// owns the lock. Users should not modify this key or the lock may exhibit
	// undefined behavior.
	Key []byte `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
}

func (m *LockResponse) Reset()                    { *m = LockResponse{} }
func (m *LockResponse) String() string            { return proto.CompactTextString(m) }
func (*LockResponse) ProtoMessage()               {}
func (*LockResponse) Descriptor() ([]byte, []int) { return fileDescriptorV3Lock, []int{1} }

func (m *LockResponse) GetHeader() *etcdserverpb.ResponseHeader {
	if m != nil {
		return m.Header
	}
	return nil
}

func (m *LockResponse) GetKey() []byte {
	if m != nil {
		return m.Key
	}
	return nil
}

type UnlockRequest struct {
	// key is the lock ownership key granted by Lock.
	Key []byte `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
}

func (m *UnlockRequest) Reset()                    { *m = UnlockRequest{} }
func (m *UnlockRequest) String() string            { return proto.CompactTextString(m) }
func (*UnlockRequest) ProtoMessage()               {}
func (*UnlockRequest) Descriptor() ([]byte, []int) { return fileDescriptorV3Lock, []int{2} }

func (m *UnlockRequest) GetKey() []byte {
	if m != nil {
		return m.Key
	}
	return nil
}

type UnlockResponse struct {
	Header *etcdserverpb.ResponseHeader `protobuf:"bytes,1,opt,name=header" json:"header,omitempty"`
}

func (m *UnlockResponse) Reset()                    { *m = UnlockResponse{} }
func (m *UnlockResponse) String() string            { return proto.CompactTextString(m) }
func (*UnlockResponse) ProtoMessage()               {}
func (*UnlockResponse) Descriptor() ([]byte, []int) { return fileDescriptorV3Lock, []int{3} }

func (m *UnlockResponse) GetHeader() *etcdserverpb.ResponseHeader {
	if m != nil {
		return m.Header
	}
	return nil
}

func init() {
	proto.RegisterType((*LockRequest)(nil), "v3lockpb.LockRequest")
	proto.RegisterType((*LockResponse)(nil), "v3lockpb.LockResponse")
	proto.RegisterType((*UnlockRequest)(nil), "v3lockpb.UnlockRequest")
	proto.RegisterType((*UnlockResponse)(nil), "v3lockpb.UnlockResponse")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for Lock service

type LockClient interface {
	// Lock acquires a distributed shared lock on a given named lock.
	// On success, it will return a unique key that exists so long as the
	// lock is held by the caller. This key can be used in conjunction with
	// transactions to safely ensure updates to etcd only occur while holding
	// lock ownership. The lock is held until Unlock is called on the key or the
	// lease associate with the owner expires.
	Lock(ctx context.Context, in *LockRequest, opts ...grpc.CallOption) (*LockResponse, error)
	// TryLock is different from Lock, TryLock locks the mutex will not be blocked
	// any time. If the mutex is occupied while trying to acquire the lock,
	// this function will return an error immediately.
	TryLock(ctx context.Context, in *LockRequest, opts ...grpc.CallOption) (*LockResponse, error)
	// Unlock takes a key returned by Lock and releases the hold on lock. The
	// next Lock caller waiting for the lock will then be woken up and given
	// ownership of the lock.
	Unlock(ctx context.Context, in *UnlockRequest, opts ...grpc.CallOption) (*UnlockResponse, error)
}

type lockClient struct {
	cc *grpc.ClientConn
}

func NewLockClient(cc *grpc.ClientConn) LockClient {
	return &lockClient{cc}
}

func (c *lockClient) Lock(ctx context.Context, in *LockRequest, opts ...grpc.CallOption) (*LockResponse, error) {
	out := new(LockResponse)
	err := grpc.Invoke(ctx, "/v3lockpb.Lock/Lock", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *lockClient) TryLock(ctx context.Context, in *LockRequest, opts ...grpc.CallOption) (*LockResponse, error) {
	out := new(LockResponse)
	err := grpc.Invoke(ctx, "/v3lockpb.Lock/TryLock", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *lockClient) Unlock(ctx context.Context, in *UnlockRequest, opts ...grpc.CallOption) (*UnlockResponse, error) {
	out := new(UnlockResponse)
	err := grpc.Invoke(ctx, "/v3lockpb.Lock/Unlock", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for Lock service

type LockServer interface {
	// Lock acquires a distributed shared lock on a given named lock.
	// On success, it will return a unique key that exists so long as the
	// lock is held by the caller. This key can be used in conjunction with
	// transactions to safely ensure updates to etcd only occur while holding
	// lock ownership. The lock is held until Unlock is called on the key or the
	// lease associate with the owner expires.
	Lock(context.Context, *LockRequest) (*LockResponse, error)
	// TryLock is different from Lock, TryLock locks the mutex will not be blocked
	// any time. If the mutex is occupied while trying to acquire the lock,
	// this function will return an error immediately.
	TryLock(context.Context, *LockRequest) (*LockResponse, error)
	// Unlock takes a key returned by Lock and releases the hold on lock. The
	// next Lock caller waiting for the lock will then be woken up and given
	// ownership of the lock.
	Unlock(context.Context, *UnlockRequest) (*UnlockResponse, error)
}

func RegisterLockServer(s *grpc.Server, srv LockServer) {
	s.RegisterService(&_Lock_serviceDesc, srv)
}

func _Lock_Lock_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LockRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LockServer).Lock(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/v3lockpb.Lock/Lock",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LockServer).Lock(ctx, req.(*LockRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Lock_TryLock_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LockRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LockServer).TryLock(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/v3lockpb.Lock/TryLock",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LockServer).TryLock(ctx, req.(*LockRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Lock_Unlock_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UnlockRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LockServer).Unlock(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/v3lockpb.Lock/Unlock",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LockServer).Unlock(ctx, req.(*UnlockRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Lock_serviceDesc = grpc.ServiceDesc{
	ServiceName: "v3lockpb.Lock",
	HandlerType: (*LockServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Lock",
			Handler:    _Lock_Lock_Handler,
		},
		{
			MethodName: "TryLock",
			Handler:    _Lock_TryLock_Handler,
		},
		{
			MethodName: "Unlock",
			Handler:    _Lock_Unlock_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "v3lock.proto",
}

func (m *LockRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *LockRequest) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Name) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintV3Lock(dAtA, i, uint64(len(m.Name)))
		i += copy(dAtA[i:], m.Name)
	}
	if m.Lease != 0 {
		dAtA[i] = 0x10
		i++
		i = encodeVarintV3Lock(dAtA, i, uint64(m.Lease))
	}
	return i, nil
}

func (m *LockResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *LockResponse) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Header != nil {
		dAtA[i] = 0xa
		i++
		i = encodeVarintV3Lock(dAtA, i, uint64(m.Header.Size()))
		n1, err := m.Header.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n1
	}
	if len(m.Key) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintV3Lock(dAtA, i, uint64(len(m.Key)))
		i += copy(dAtA[i:], m.Key)
	}
	return i, nil
}

func (m *UnlockRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *UnlockRequest) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Key) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintV3Lock(dAtA, i, uint64(len(m.Key)))
		i += copy(dAtA[i:], m.Key)
	}
	return i, nil
}

func (m *UnlockResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *UnlockResponse) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Header != nil {
		dAtA[i] = 0xa
		i++
		i = encodeVarintV3Lock(dAtA, i, uint64(m.Header.Size()))
		n2, err := m.Header.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n2
	}
	return i, nil
}

func encodeVarintV3Lock(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *LockRequest) Size() (n int) {
	var l int
	_ = l
	l = len(m.Name)
	if l > 0 {
		n += 1 + l + sovV3Lock(uint64(l))
	}
	if m.Lease != 0 {
		n += 1 + sovV3Lock(uint64(m.Lease))
	}
	return n
}

func (m *LockResponse) Size() (n int) {
	var l int
	_ = l
	if m.Header != nil {
		l = m.Header.Size()
		n += 1 + l + sovV3Lock(uint64(l))
	}
	l = len(m.Key)
	if l > 0 {
		n += 1 + l + sovV3Lock(uint64(l))
	}
	return n
}

func (m *UnlockRequest) Size() (n int) {
	var l int
	_ = l
	l = len(m.Key)
	if l > 0 {
		n += 1 + l + sovV3Lock(uint64(l))
	}
	return n
}

func (m *UnlockResponse) Size() (n int) {
	var l int
	_ = l
	if m.Header != nil {
		l = m.Header.Size()
		n += 1 + l + sovV3Lock(uint64(l))
	}
	return n
}

func sovV3Lock(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozV3Lock(x uint64) (n int) {
	return sovV3Lock(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *LockRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowV3Lock
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: LockRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: LockRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Name", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowV3Lock
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthV3Lock
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Name = append(m.Name[:0], dAtA[iNdEx:postIndex]...)
			if m.Name == nil {
				m.Name = []byte{}
			}
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Lease", wireType)
			}
			m.Lease = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowV3Lock
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Lease |= (int64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipV3Lock(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthV3Lock
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *LockResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowV3Lock
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: LockResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: LockResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Header", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowV3Lock
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthV3Lock
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Header == nil {
				m.Header = &etcdserverpb.ResponseHeader{}
			}
			if err := m.Header.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Key", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowV3Lock
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthV3Lock
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Key = append(m.Key[:0], dAtA[iNdEx:postIndex]...)
			if m.Key == nil {
				m.Key = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipV3Lock(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthV3Lock
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *UnlockRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowV3Lock
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: UnlockRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: UnlockRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Key", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowV3Lock
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthV3Lock
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Key = append(m.Key[:0], dAtA[iNdEx:postIndex]...)
			if m.Key == nil {
				m.Key = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipV3Lock(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthV3Lock
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *UnlockResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowV3Lock
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: UnlockResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: UnlockResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Header", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowV3Lock
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthV3Lock
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Header == nil {
				m.Header = &etcdserverpb.ResponseHeader{}
			}
			if err := m.Header.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipV3Lock(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthV3Lock
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipV3Lock(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowV3Lock
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowV3Lock
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowV3Lock
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthV3Lock
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowV3Lock
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipV3Lock(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthV3Lock = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowV3Lock   = fmt.Errorf("proto: integer overflow")
)

func init() { proto.RegisterFile("v3lock.proto", fileDescriptorV3Lock) }

var fileDescriptorV3Lock = []byte{
	// 352 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xa4, 0x92, 0x41, 0x4b, 0xc3, 0x30,
	0x14, 0x80, 0xcd, 0x36, 0xa7, 0x64, 0x9d, 0x8e, 0xb0, 0x69, 0xa9, 0xa3, 0xcc, 0x1c, 0x64, 0x78,
	0x68, 0x60, 0x13, 0x04, 0x8f, 0x1e, 0xc4, 0x83, 0x20, 0x14, 0xa7, 0xe7, 0xae, 0x7b, 0xd4, 0xb1,
	0x9a, 0xd4, 0xb4, 0x1b, 0xec, 0xea, 0xc5, 0x1f, 0xe0, 0xc5, 0x9f, 0xe4, 0x51, 0xf0, 0x0f, 0xc8,
	0xf4, 0x87, 0x48, 0x93, 0x76, 0xab, 0x7a, 0xd3, 0x4b, 0xfa, 0x92, 0xf7, 0xbd, 0xaf, 0x79, 0x8f,
	0x60, 0x63, 0xd6, 0x0f, 0x85, 0x3f, 0x71, 0x22, 0x29, 0x12, 0x41, 0x36, 0xf5, 0x2e, 0x1a, 0x5a,
	0xcd, 0x40, 0x04, 0x42, 0x1d, 0xb2, 0x34, 0xd2, 0x79, 0xeb, 0x00, 0x12, 0x7f, 0xc4, 0xd2, 0x25,
	0x06, 0x39, 0x03, 0x59, 0x08, 0xa3, 0x21, 0x93, 0x91, 0x9f, 0x71, 0xed, 0x40, 0x88, 0x20, 0x04,
	0xe6, 0x45, 0x63, 0xe6, 0x71, 0x2e, 0x12, 0x2f, 0x19, 0x0b, 0x1e, 0xeb, 0x2c, 0x3d, 0xc6, 0xb5,
	0x0b, 0xe1, 0x4f, 0x5c, 0xb8, 0x9f, 0x42, 0x9c, 0x10, 0x82, 0x2b, 0xdc, 0xbb, 0x03, 0x13, 0x75,
	0x50, 0xd7, 0x70, 0x55, 0x4c, 0x9a, 0x78, 0x3d, 0x04, 0x2f, 0x06, 0xb3, 0xd4, 0x41, 0xdd, 0xb2,
	0xab, 0x37, 0xf4, 0x1a, 0x1b, 0xba, 0x30, 0x8e, 0x04, 0x8f, 0x81, 0x1c, 0xe1, 0xea, 0x2d, 0x78,
	0x23, 0x90, 0xaa, 0xb6, 0xd6, 0x6b, 0x3b, 0xc5, 0xfb, 0x38, 0x39, 0x77, 0xae, 0x18, 0x37, 0x63,
	0x49, 0x03, 0x97, 0x27, 0x30, 0x57, 0x66, 0xc3, 0x4d, 0x43, 0xba, 0x8f, 0xeb, 0x03, 0x1e, 0x16,
	0xae, 0x94, 0x21, 0x68, 0x85, 0x9c, 0xe1, 0xad, 0x1c, 0xf9, 0xcf, 0xcf, 0x7b, 0x8f, 0x25, 0x5c,
	0x49, 0x7b, 0x20, 0x97, 0xd9, 0xb7, 0xe5, 0xe4, 0x33, 0x77, 0x0a, 0x43, 0xb1, 0x76, 0x7e, 0x1e,
	0x6b, 0x1b, 0x35, 0x1f, 0xde, 0x3e, 0x9f, 0x4a, 0x84, 0xd6, 0xd9, 0xac, 0xcf, 0x52, 0x40, 0x2d,
	0x27, 0xe8, 0x90, 0x0c, 0xf0, 0xc6, 0x95, 0x9c, 0xff, 0xc5, 0xb9, 0xa7, 0x9c, 0x2d, 0xda, 0x58,
	0x3a, 0x13, 0x39, 0xcf, 0xb5, 0x37, 0xb8, 0xaa, 0x1b, 0x27, 0xbb, 0xab, 0xf2, 0x6f, 0xd3, 0xb2,
	0xcc, 0xdf, 0x89, 0xcc, 0x6c, 0x29, 0x73, 0x93, 0x6e, 0x2f, 0xcd, 0x53, 0x9e, 0x89, 0x4f, 0x1b,
	0x2f, 0x0b, 0x1b, 0xbd, 0x2e, 0x6c, 0xf4, 0xbe, 0xb0, 0xd1, 0xf3, 0x87, 0xbd, 0x36, 0xac, 0xaa,
	0xe7, 0xd1, 0xff, 0x0a, 0x00, 0x00, 0xff, 0xff, 0x0d, 0x11, 0x26, 0x35, 0x94, 0x02, 0x00, 0x00,
}
