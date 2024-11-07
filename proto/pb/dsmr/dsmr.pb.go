// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        (unknown)
// source: dsmr/dsmr.proto

package dsmr

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GetChunkRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ChunkId []byte `protobuf:"bytes,1,opt,name=chunk_id,json=chunkId,proto3" json:"chunk_id,omitempty"`
	Expiry  int64  `protobuf:"varint,2,opt,name=expiry,proto3" json:"expiry,omitempty"`
}

func (x *GetChunkRequest) Reset() {
	*x = GetChunkRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_dsmr_dsmr_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetChunkRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetChunkRequest) ProtoMessage() {}

func (x *GetChunkRequest) ProtoReflect() protoreflect.Message {
	mi := &file_dsmr_dsmr_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetChunkRequest.ProtoReflect.Descriptor instead.
func (*GetChunkRequest) Descriptor() ([]byte, []int) {
	return file_dsmr_dsmr_proto_rawDescGZIP(), []int{0}
}

func (x *GetChunkRequest) GetChunkId() []byte {
	if x != nil {
		return x.ChunkId
	}
	return nil
}

func (x *GetChunkRequest) GetExpiry() int64 {
	if x != nil {
		return x.Expiry
	}
	return 0
}

type GetChunkResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Chunk []byte `protobuf:"bytes,1,opt,name=chunk,proto3" json:"chunk,omitempty"`
}

func (x *GetChunkResponse) Reset() {
	*x = GetChunkResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_dsmr_dsmr_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetChunkResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetChunkResponse) ProtoMessage() {}

func (x *GetChunkResponse) ProtoReflect() protoreflect.Message {
	mi := &file_dsmr_dsmr_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetChunkResponse.ProtoReflect.Descriptor instead.
func (*GetChunkResponse) Descriptor() ([]byte, []int) {
	return file_dsmr_dsmr_proto_rawDescGZIP(), []int{1}
}

func (x *GetChunkResponse) GetChunk() []byte {
	if x != nil {
		return x.Chunk
	}
	return nil
}

type GetChunkSignatureRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Chunk []byte `protobuf:"bytes,1,opt,name=chunk,proto3" json:"chunk,omitempty"`
}

func (x *GetChunkSignatureRequest) Reset() {
	*x = GetChunkSignatureRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_dsmr_dsmr_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetChunkSignatureRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetChunkSignatureRequest) ProtoMessage() {}

func (x *GetChunkSignatureRequest) ProtoReflect() protoreflect.Message {
	mi := &file_dsmr_dsmr_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetChunkSignatureRequest.ProtoReflect.Descriptor instead.
func (*GetChunkSignatureRequest) Descriptor() ([]byte, []int) {
	return file_dsmr_dsmr_proto_rawDescGZIP(), []int{2}
}

func (x *GetChunkSignatureRequest) GetChunk() []byte {
	if x != nil {
		return x.Chunk
	}
	return nil
}

type GetChunkSignatureResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// TODO are these fields needed?
	ChunkId   []byte `protobuf:"bytes,1,opt,name=chunk_id,json=chunkId,proto3" json:"chunk_id,omitempty"`
	Producer  []byte `protobuf:"bytes,2,opt,name=producer,proto3" json:"producer,omitempty"`
	Expiry    int64  `protobuf:"varint,3,opt,name=expiry,proto3" json:"expiry,omitempty"`
	Signer    []byte `protobuf:"bytes,4,opt,name=signer,proto3" json:"signer,omitempty"`
	Signature []byte `protobuf:"bytes,5,opt,name=signature,proto3" json:"signature,omitempty"`
}

func (x *GetChunkSignatureResponse) Reset() {
	*x = GetChunkSignatureResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_dsmr_dsmr_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetChunkSignatureResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetChunkSignatureResponse) ProtoMessage() {}

func (x *GetChunkSignatureResponse) ProtoReflect() protoreflect.Message {
	mi := &file_dsmr_dsmr_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetChunkSignatureResponse.ProtoReflect.Descriptor instead.
func (*GetChunkSignatureResponse) Descriptor() ([]byte, []int) {
	return file_dsmr_dsmr_proto_rawDescGZIP(), []int{3}
}

func (x *GetChunkSignatureResponse) GetChunkId() []byte {
	if x != nil {
		return x.ChunkId
	}
	return nil
}

func (x *GetChunkSignatureResponse) GetProducer() []byte {
	if x != nil {
		return x.Producer
	}
	return nil
}

func (x *GetChunkSignatureResponse) GetExpiry() int64 {
	if x != nil {
		return x.Expiry
	}
	return 0
}

func (x *GetChunkSignatureResponse) GetSigner() []byte {
	if x != nil {
		return x.Signer
	}
	return nil
}

func (x *GetChunkSignatureResponse) GetSignature() []byte {
	if x != nil {
		return x.Signature
	}
	return nil
}

type ChunkCertificateGossip struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ChunkCertificate []byte `protobuf:"bytes,1,opt,name=chunk_certificate,json=chunkCertificate,proto3" json:"chunk_certificate,omitempty"`
}

func (x *ChunkCertificateGossip) Reset() {
	*x = ChunkCertificateGossip{}
	if protoimpl.UnsafeEnabled {
		mi := &file_dsmr_dsmr_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ChunkCertificateGossip) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChunkCertificateGossip) ProtoMessage() {}

func (x *ChunkCertificateGossip) ProtoReflect() protoreflect.Message {
	mi := &file_dsmr_dsmr_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ChunkCertificateGossip.ProtoReflect.Descriptor instead.
func (*ChunkCertificateGossip) Descriptor() ([]byte, []int) {
	return file_dsmr_dsmr_proto_rawDescGZIP(), []int{4}
}

func (x *ChunkCertificateGossip) GetChunkCertificate() []byte {
	if x != nil {
		return x.ChunkCertificate
	}
	return nil
}

var File_dsmr_dsmr_proto protoreflect.FileDescriptor

var file_dsmr_dsmr_proto_rawDesc = []byte{
	0x0a, 0x0f, 0x64, 0x73, 0x6d, 0x72, 0x2f, 0x64, 0x73, 0x6d, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x04, 0x64, 0x73, 0x6d, 0x72, 0x22, 0x44, 0x0a, 0x0f, 0x47, 0x65, 0x74, 0x43, 0x68,
	0x75, 0x6e, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x19, 0x0a, 0x08, 0x63, 0x68,
	0x75, 0x6e, 0x6b, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x07, 0x63, 0x68,
	0x75, 0x6e, 0x6b, 0x49, 0x64, 0x12, 0x16, 0x0a, 0x06, 0x65, 0x78, 0x70, 0x69, 0x72, 0x79, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x06, 0x65, 0x78, 0x70, 0x69, 0x72, 0x79, 0x22, 0x28, 0x0a,
	0x10, 0x47, 0x65, 0x74, 0x43, 0x68, 0x75, 0x6e, 0x6b, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x14, 0x0a, 0x05, 0x63, 0x68, 0x75, 0x6e, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c,
	0x52, 0x05, 0x63, 0x68, 0x75, 0x6e, 0x6b, 0x22, 0x30, 0x0a, 0x18, 0x47, 0x65, 0x74, 0x43, 0x68,
	0x75, 0x6e, 0x6b, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x63, 0x68, 0x75, 0x6e, 0x6b, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x05, 0x63, 0x68, 0x75, 0x6e, 0x6b, 0x22, 0xa0, 0x01, 0x0a, 0x19, 0x47, 0x65,
	0x74, 0x43, 0x68, 0x75, 0x6e, 0x6b, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x63, 0x68, 0x75, 0x6e, 0x6b,
	0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x07, 0x63, 0x68, 0x75, 0x6e, 0x6b,
	0x49, 0x64, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x65, 0x72, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0c, 0x52, 0x08, 0x70, 0x72, 0x6f, 0x64, 0x75, 0x63, 0x65, 0x72, 0x12, 0x16,
	0x0a, 0x06, 0x65, 0x78, 0x70, 0x69, 0x72, 0x79, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x06,
	0x65, 0x78, 0x70, 0x69, 0x72, 0x79, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x72,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x06, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x72, 0x12, 0x1c,
	0x0a, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x0c, 0x52, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x22, 0x45, 0x0a, 0x16,
	0x43, 0x68, 0x75, 0x6e, 0x6b, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65,
	0x47, 0x6f, 0x73, 0x73, 0x69, 0x70, 0x12, 0x2b, 0x0a, 0x11, 0x63, 0x68, 0x75, 0x6e, 0x6b, 0x5f,
	0x63, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0c, 0x52, 0x10, 0x63, 0x68, 0x75, 0x6e, 0x6b, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63,
	0x61, 0x74, 0x65, 0x42, 0x2c, 0x5a, 0x2a, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x61, 0x76, 0x61, 0x2d, 0x6c, 0x61, 0x62, 0x73, 0x2f, 0x68, 0x79, 0x70, 0x65, 0x72,
	0x73, 0x64, 0x6b, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x70, 0x62, 0x2f, 0x64, 0x73, 0x6d,
	0x72, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_dsmr_dsmr_proto_rawDescOnce sync.Once
	file_dsmr_dsmr_proto_rawDescData = file_dsmr_dsmr_proto_rawDesc
)

func file_dsmr_dsmr_proto_rawDescGZIP() []byte {
	file_dsmr_dsmr_proto_rawDescOnce.Do(func() {
		file_dsmr_dsmr_proto_rawDescData = protoimpl.X.CompressGZIP(file_dsmr_dsmr_proto_rawDescData)
	})
	return file_dsmr_dsmr_proto_rawDescData
}

var file_dsmr_dsmr_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_dsmr_dsmr_proto_goTypes = []interface{}{
	(*GetChunkRequest)(nil),           // 0: dsmr.GetChunkRequest
	(*GetChunkResponse)(nil),          // 1: dsmr.GetChunkResponse
	(*GetChunkSignatureRequest)(nil),  // 2: dsmr.GetChunkSignatureRequest
	(*GetChunkSignatureResponse)(nil), // 3: dsmr.GetChunkSignatureResponse
	(*ChunkCertificateGossip)(nil),    // 4: dsmr.ChunkCertificateGossip
}
var file_dsmr_dsmr_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_dsmr_dsmr_proto_init() }
func file_dsmr_dsmr_proto_init() {
	if File_dsmr_dsmr_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_dsmr_dsmr_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetChunkRequest); i {
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
		file_dsmr_dsmr_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetChunkResponse); i {
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
		file_dsmr_dsmr_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetChunkSignatureRequest); i {
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
		file_dsmr_dsmr_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetChunkSignatureResponse); i {
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
		file_dsmr_dsmr_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ChunkCertificateGossip); i {
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
			RawDescriptor: file_dsmr_dsmr_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_dsmr_dsmr_proto_goTypes,
		DependencyIndexes: file_dsmr_dsmr_proto_depIdxs,
		MessageInfos:      file_dsmr_dsmr_proto_msgTypes,
	}.Build()
	File_dsmr_dsmr_proto = out.File
	file_dsmr_dsmr_proto_rawDesc = nil
	file_dsmr_dsmr_proto_goTypes = nil
	file_dsmr_dsmr_proto_depIdxs = nil
}
