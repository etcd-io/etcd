// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.19.4
// source: api/authpb/auth.proto

package authpb

import (
	reflect "reflect"
	sync "sync"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Permission_Type int32

const (
	Permission_READ      Permission_Type = 0
	Permission_WRITE     Permission_Type = 1
	Permission_READWRITE Permission_Type = 2
)

// Enum value maps for Permission_Type.
var (
	Permission_Type_name = map[int32]string{
		0: "READ",
		1: "WRITE",
		2: "READWRITE",
	}
	Permission_Type_value = map[string]int32{
		"READ":      0,
		"WRITE":     1,
		"READWRITE": 2,
	}
)

func (x Permission_Type) Enum() *Permission_Type {
	p := new(Permission_Type)
	*p = x
	return p
}

func (x Permission_Type) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Permission_Type) Descriptor() protoreflect.EnumDescriptor {
	return file_api_authpb_auth_proto_enumTypes[0].Descriptor()
}

func (Permission_Type) Type() protoreflect.EnumType {
	return &file_api_authpb_auth_proto_enumTypes[0]
}

func (x Permission_Type) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Permission_Type.Descriptor instead.
func (Permission_Type) EnumDescriptor() ([]byte, []int) {
	return file_api_authpb_auth_proto_rawDescGZIP(), []int{2, 0}
}

type UserAddOptions struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	NoPassword bool `protobuf:"varint,1,opt,name=no_password,json=noPassword,proto3" json:"no_password,omitempty"`
}

func (x *UserAddOptions) Reset() {
	*x = UserAddOptions{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_authpb_auth_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UserAddOptions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UserAddOptions) ProtoMessage() {}

func (x *UserAddOptions) ProtoReflect() protoreflect.Message {
	mi := &file_api_authpb_auth_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UserAddOptions.ProtoReflect.Descriptor instead.
func (*UserAddOptions) Descriptor() ([]byte, []int) {
	return file_api_authpb_auth_proto_rawDescGZIP(), []int{0}
}

func (x *UserAddOptions) GetNoPassword() bool {
	if x != nil {
		return x.NoPassword
	}
	return false
}

// User is a single entry in the bucket authUsers
type User struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name     []byte          `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Password []byte          `protobuf:"bytes,2,opt,name=password,proto3" json:"password,omitempty"`
	Roles    []string        `protobuf:"bytes,3,rep,name=roles,proto3" json:"roles,omitempty"`
	Options  *UserAddOptions `protobuf:"bytes,4,opt,name=options,proto3" json:"options,omitempty"`
}

func (x *User) Reset() {
	*x = User{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_authpb_auth_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *User) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*User) ProtoMessage() {}

func (x *User) ProtoReflect() protoreflect.Message {
	mi := &file_api_authpb_auth_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use User.ProtoReflect.Descriptor instead.
func (*User) Descriptor() ([]byte, []int) {
	return file_api_authpb_auth_proto_rawDescGZIP(), []int{1}
}

func (x *User) GetName() []byte {
	if x != nil {
		return x.Name
	}
	return nil
}

func (x *User) GetPassword() []byte {
	if x != nil {
		return x.Password
	}
	return nil
}

func (x *User) GetRoles() []string {
	if x != nil {
		return x.Roles
	}
	return nil
}

func (x *User) GetOptions() *UserAddOptions {
	if x != nil {
		return x.Options
	}
	return nil
}

// Permission is a single entity
type Permission struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PermType Permission_Type `protobuf:"varint,1,opt,name=permType,proto3,enum=authpb.Permission_Type" json:"permType,omitempty"`
	Key      []byte          `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
	RangeEnd []byte          `protobuf:"bytes,3,opt,name=range_end,json=rangeEnd,proto3" json:"range_end,omitempty"`
}

func (x *Permission) Reset() {
	*x = Permission{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_authpb_auth_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Permission) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Permission) ProtoMessage() {}

func (x *Permission) ProtoReflect() protoreflect.Message {
	mi := &file_api_authpb_auth_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Permission.ProtoReflect.Descriptor instead.
func (*Permission) Descriptor() ([]byte, []int) {
	return file_api_authpb_auth_proto_rawDescGZIP(), []int{2}
}

func (x *Permission) GetPermType() Permission_Type {
	if x != nil {
		return x.PermType
	}
	return Permission_READ
}

func (x *Permission) GetKey() []byte {
	if x != nil {
		return x.Key
	}
	return nil
}

func (x *Permission) GetRangeEnd() []byte {
	if x != nil {
		return x.RangeEnd
	}
	return nil
}

// Role is a single entry in the bucket authRoles
type Role struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name          []byte        `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	KeyPermission []*Permission `protobuf:"bytes,2,rep,name=keyPermission,proto3" json:"keyPermission,omitempty"`
}

func (x *Role) Reset() {
	*x = Role{}
	if protoimpl.UnsafeEnabled {
		mi := &file_api_authpb_auth_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Role) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Role) ProtoMessage() {}

func (x *Role) ProtoReflect() protoreflect.Message {
	mi := &file_api_authpb_auth_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Role.ProtoReflect.Descriptor instead.
func (*Role) Descriptor() ([]byte, []int) {
	return file_api_authpb_auth_proto_rawDescGZIP(), []int{3}
}

func (x *Role) GetName() []byte {
	if x != nil {
		return x.Name
	}
	return nil
}

func (x *Role) GetKeyPermission() []*Permission {
	if x != nil {
		return x.KeyPermission
	}
	return nil
}

var File_api_authpb_auth_proto protoreflect.FileDescriptor

var file_api_authpb_auth_proto_rawDesc = []byte{
	0x0a, 0x15, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x75, 0x74, 0x68, 0x70, 0x62, 0x2f, 0x61, 0x75, 0x74,
	0x68, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x06, 0x61, 0x75, 0x74, 0x68, 0x70, 0x62, 0x22,
	0x31, 0x0a, 0x0e, 0x55, 0x73, 0x65, 0x72, 0x41, 0x64, 0x64, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x12, 0x1f, 0x0a, 0x0b, 0x6e, 0x6f, 0x5f, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0a, 0x6e, 0x6f, 0x50, 0x61, 0x73, 0x73, 0x77, 0x6f,
	0x72, 0x64, 0x22, 0x7e, 0x0a, 0x04, 0x55, 0x73, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1a,
	0x0a, 0x08, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c,
	0x52, 0x08, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x12, 0x14, 0x0a, 0x05, 0x72, 0x6f,
	0x6c, 0x65, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x09, 0x52, 0x05, 0x72, 0x6f, 0x6c, 0x65, 0x73,
	0x12, 0x30, 0x0a, 0x07, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x16, 0x2e, 0x61, 0x75, 0x74, 0x68, 0x70, 0x62, 0x2e, 0x55, 0x73, 0x65, 0x72, 0x41,
	0x64, 0x64, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x07, 0x6f, 0x70, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x22, 0x9c, 0x01, 0x0a, 0x0a, 0x50, 0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f,
	0x6e, 0x12, 0x33, 0x0a, 0x08, 0x70, 0x65, 0x72, 0x6d, 0x54, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0e, 0x32, 0x17, 0x2e, 0x61, 0x75, 0x74, 0x68, 0x70, 0x62, 0x2e, 0x50, 0x65, 0x72,
	0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x2e, 0x54, 0x79, 0x70, 0x65, 0x52, 0x08, 0x70, 0x65,
	0x72, 0x6d, 0x54, 0x79, 0x70, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x1b, 0x0a, 0x09, 0x72, 0x61, 0x6e, 0x67,
	0x65, 0x5f, 0x65, 0x6e, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x08, 0x72, 0x61, 0x6e,
	0x67, 0x65, 0x45, 0x6e, 0x64, 0x22, 0x2a, 0x0a, 0x04, 0x54, 0x79, 0x70, 0x65, 0x12, 0x08, 0x0a,
	0x04, 0x52, 0x45, 0x41, 0x44, 0x10, 0x00, 0x12, 0x09, 0x0a, 0x05, 0x57, 0x52, 0x49, 0x54, 0x45,
	0x10, 0x01, 0x12, 0x0d, 0x0a, 0x09, 0x52, 0x45, 0x41, 0x44, 0x57, 0x52, 0x49, 0x54, 0x45, 0x10,
	0x02, 0x22, 0x54, 0x0a, 0x04, 0x52, 0x6f, 0x6c, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x38, 0x0a,
	0x0d, 0x6b, 0x65, 0x79, 0x50, 0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x02,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x61, 0x75, 0x74, 0x68, 0x70, 0x62, 0x2e, 0x50, 0x65,
	0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x0d, 0x6b, 0x65, 0x79, 0x50, 0x65, 0x72,
	0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x42, 0x1c, 0x5a, 0x1a, 0x67, 0x6f, 0x2e, 0x65, 0x74,
	0x63, 0x64, 0x2e, 0x69, 0x6f, 0x2f, 0x65, 0x74, 0x63, 0x64, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61,
	0x75, 0x74, 0x68, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_api_authpb_auth_proto_rawDescOnce sync.Once
	file_api_authpb_auth_proto_rawDescData = file_api_authpb_auth_proto_rawDesc
)

func file_api_authpb_auth_proto_rawDescGZIP() []byte {
	file_api_authpb_auth_proto_rawDescOnce.Do(func() {
		file_api_authpb_auth_proto_rawDescData = protoimpl.X.CompressGZIP(file_api_authpb_auth_proto_rawDescData)
	})
	return file_api_authpb_auth_proto_rawDescData
}

var file_api_authpb_auth_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_api_authpb_auth_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_api_authpb_auth_proto_goTypes = []interface{}{
	(Permission_Type)(0),   // 0: authpb.Permission.Type
	(*UserAddOptions)(nil), // 1: authpb.UserAddOptions
	(*User)(nil),           // 2: authpb.User
	(*Permission)(nil),     // 3: authpb.Permission
	(*Role)(nil),           // 4: authpb.Role
}
var file_api_authpb_auth_proto_depIdxs = []int32{
	1, // 0: authpb.User.options:type_name -> authpb.UserAddOptions
	0, // 1: authpb.Permission.permType:type_name -> authpb.Permission.Type
	3, // 2: authpb.Role.keyPermission:type_name -> authpb.Permission
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_api_authpb_auth_proto_init() }
func file_api_authpb_auth_proto_init() {
	if File_api_authpb_auth_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_api_authpb_auth_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UserAddOptions); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_api_authpb_auth_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*User); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_api_authpb_auth_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Permission); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_api_authpb_auth_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Role); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_api_authpb_auth_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_api_authpb_auth_proto_goTypes,
		DependencyIndexes: file_api_authpb_auth_proto_depIdxs,
		EnumInfos:         file_api_authpb_auth_proto_enumTypes,
		MessageInfos:      file_api_authpb_auth_proto_msgTypes,
	}.Build()
	File_api_authpb_auth_proto = out.File
	file_api_authpb_auth_proto_rawDesc = nil
	file_api_authpb_auth_proto_goTypes = nil
	file_api_authpb_auth_proto_depIdxs = nil
}
