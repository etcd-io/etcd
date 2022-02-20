// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.19.4
// source: server/storage/wal/walpb/record.proto

package walpb

import (
	reflect "reflect"
	sync "sync"

	raftpb "go.etcd.io/etcd/raft/v3/raftpb"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Record struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type *int64  `protobuf:"varint,1,opt,name=type" json:"type,omitempty"`
	Crc  *uint32 `protobuf:"varint,2,opt,name=crc" json:"crc,omitempty"`
	Data []byte  `protobuf:"bytes,3,opt,name=data" json:"data,omitempty"`
}

func (x *Record) Reset() {
	*x = Record{}
	if protoimpl.UnsafeEnabled {
		mi := &file_server_storage_wal_walpb_record_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Record) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Record) ProtoMessage() {}

func (x *Record) ProtoReflect() protoreflect.Message {
	mi := &file_server_storage_wal_walpb_record_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Record.ProtoReflect.Descriptor instead.
func (*Record) Descriptor() ([]byte, []int) {
	return file_server_storage_wal_walpb_record_proto_rawDescGZIP(), []int{0}
}

func (x *Record) GetType() int64 {
	if x != nil && x.Type != nil {
		return *x.Type
	}
	return 0
}

func (x *Record) GetCrc() uint32 {
	if x != nil && x.Crc != nil {
		return *x.Crc
	}
	return 0
}

func (x *Record) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

// Keep in sync with raftpb.SnapshotMetadata.
type Snapshot struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Index *uint64 `protobuf:"varint,1,opt,name=index" json:"index,omitempty"`
	Term  *uint64 `protobuf:"varint,2,opt,name=term" json:"term,omitempty"`
	// Field populated since >=etcd-3.5.0.
	ConfState *raftpb.ConfState `protobuf:"bytes,3,opt,name=conf_state,json=confState" json:"conf_state,omitempty"`
}

func (x *Snapshot) Reset() {
	*x = Snapshot{}
	if protoimpl.UnsafeEnabled {
		mi := &file_server_storage_wal_walpb_record_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Snapshot) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Snapshot) ProtoMessage() {}

func (x *Snapshot) ProtoReflect() protoreflect.Message {
	mi := &file_server_storage_wal_walpb_record_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Snapshot.ProtoReflect.Descriptor instead.
func (*Snapshot) Descriptor() ([]byte, []int) {
	return file_server_storage_wal_walpb_record_proto_rawDescGZIP(), []int{1}
}

func (x *Snapshot) GetIndex() uint64 {
	if x != nil && x.Index != nil {
		return *x.Index
	}
	return 0
}

func (x *Snapshot) GetTerm() uint64 {
	if x != nil && x.Term != nil {
		return *x.Term
	}
	return 0
}

func (x *Snapshot) GetConfState() *raftpb.ConfState {
	if x != nil {
		return x.ConfState
	}
	return nil
}

var File_server_storage_wal_walpb_record_proto protoreflect.FileDescriptor

var file_server_storage_wal_walpb_record_proto_rawDesc = []byte{
	0x0a, 0x25, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x2f, 0x73, 0x74, 0x6f, 0x72, 0x61, 0x67, 0x65,
	0x2f, 0x77, 0x61, 0x6c, 0x2f, 0x77, 0x61, 0x6c, 0x70, 0x62, 0x2f, 0x72, 0x65, 0x63, 0x6f, 0x72,
	0x64, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x77, 0x61, 0x6c, 0x70, 0x62, 0x1a, 0x16,
	0x72, 0x61, 0x66, 0x74, 0x2f, 0x72, 0x61, 0x66, 0x74, 0x70, 0x62, 0x2f, 0x72, 0x61, 0x66, 0x74,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x42, 0x0a, 0x06, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64,
	0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x04,
	0x74, 0x79, 0x70, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x63, 0x72, 0x63, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0d, 0x52, 0x03, 0x63, 0x72, 0x63, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x22, 0x66, 0x0a, 0x08, 0x53, 0x6e,
	0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x12, 0x12, 0x0a, 0x04,
	0x74, 0x65, 0x72, 0x6d, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x74, 0x65, 0x72, 0x6d,
	0x12, 0x30, 0x0a, 0x0a, 0x63, 0x6f, 0x6e, 0x66, 0x5f, 0x73, 0x74, 0x61, 0x74, 0x65, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x72, 0x61, 0x66, 0x74, 0x70, 0x62, 0x2e, 0x43, 0x6f,
	0x6e, 0x66, 0x53, 0x74, 0x61, 0x74, 0x65, 0x52, 0x09, 0x63, 0x6f, 0x6e, 0x66, 0x53, 0x74, 0x61,
	0x74, 0x65, 0x42, 0x2a, 0x5a, 0x28, 0x67, 0x6f, 0x2e, 0x65, 0x74, 0x63, 0x64, 0x2e, 0x69, 0x6f,
	0x2f, 0x65, 0x74, 0x63, 0x64, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x2f, 0x73, 0x74, 0x6f,
	0x72, 0x61, 0x67, 0x65, 0x2f, 0x77, 0x61, 0x6c, 0x2f, 0x77, 0x61, 0x6c, 0x70, 0x62,
}

var (
	file_server_storage_wal_walpb_record_proto_rawDescOnce sync.Once
	file_server_storage_wal_walpb_record_proto_rawDescData = file_server_storage_wal_walpb_record_proto_rawDesc
)

func file_server_storage_wal_walpb_record_proto_rawDescGZIP() []byte {
	file_server_storage_wal_walpb_record_proto_rawDescOnce.Do(func() {
		file_server_storage_wal_walpb_record_proto_rawDescData = protoimpl.X.CompressGZIP(file_server_storage_wal_walpb_record_proto_rawDescData)
	})
	return file_server_storage_wal_walpb_record_proto_rawDescData
}

var file_server_storage_wal_walpb_record_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_server_storage_wal_walpb_record_proto_goTypes = []interface{}{
	(*Record)(nil),           // 0: walpb.Record
	(*Snapshot)(nil),         // 1: walpb.Snapshot
	(*raftpb.ConfState)(nil), // 2: raftpb.ConfState
}
var file_server_storage_wal_walpb_record_proto_depIdxs = []int32{
	2, // 0: walpb.Snapshot.conf_state:type_name -> raftpb.ConfState
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_server_storage_wal_walpb_record_proto_init() }
func file_server_storage_wal_walpb_record_proto_init() {
	if File_server_storage_wal_walpb_record_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_server_storage_wal_walpb_record_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Record); i {
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
		file_server_storage_wal_walpb_record_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Snapshot); i {
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
			RawDescriptor: file_server_storage_wal_walpb_record_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_server_storage_wal_walpb_record_proto_goTypes,
		DependencyIndexes: file_server_storage_wal_walpb_record_proto_depIdxs,
		MessageInfos:      file_server_storage_wal_walpb_record_proto_msgTypes,
	}.Build()
	File_server_storage_wal_walpb_record_proto = out.File
	file_server_storage_wal_walpb_record_proto_rawDesc = nil
	file_server_storage_wal_walpb_record_proto_goTypes = nil
	file_server_storage_wal_walpb_record_proto_depIdxs = nil
}
