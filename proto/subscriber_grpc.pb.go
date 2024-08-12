// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package __

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ExternalSubscriberClient is the client API for ExternalSubscriber service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ExternalSubscriberClient interface {
	ProcessBlock(ctx context.Context, in *BlockRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	Initialize(ctx context.Context, in *InitRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type externalSubscriberClient struct {
	cc grpc.ClientConnInterface
}

func NewExternalSubscriberClient(cc grpc.ClientConnInterface) ExternalSubscriberClient {
	return &externalSubscriberClient{cc}
}

func (c *externalSubscriberClient) ProcessBlock(ctx context.Context, in *BlockRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/subscriber.ExternalSubscriber/ProcessBlock", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *externalSubscriberClient) Initialize(ctx context.Context, in *InitRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/subscriber.ExternalSubscriber/Initialize", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ExternalSubscriberServer is the server API for ExternalSubscriber service.
// All implementations must embed UnimplementedExternalSubscriberServer
// for forward compatibility
type ExternalSubscriberServer interface {
	ProcessBlock(context.Context, *BlockRequest) (*emptypb.Empty, error)
	Initialize(context.Context, *InitRequest) (*emptypb.Empty, error)
	mustEmbedUnimplementedExternalSubscriberServer()
}

// UnimplementedExternalSubscriberServer must be embedded to have forward compatible implementations.
type UnimplementedExternalSubscriberServer struct {
}

func (UnimplementedExternalSubscriberServer) ProcessBlock(context.Context, *BlockRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ProcessBlock not implemented")
}
func (UnimplementedExternalSubscriberServer) Initialize(context.Context, *InitRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Initialize not implemented")
}
func (UnimplementedExternalSubscriberServer) mustEmbedUnimplementedExternalSubscriberServer() {}

// UnsafeExternalSubscriberServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ExternalSubscriberServer will
// result in compilation errors.
type UnsafeExternalSubscriberServer interface {
	mustEmbedUnimplementedExternalSubscriberServer()
}

func RegisterExternalSubscriberServer(s grpc.ServiceRegistrar, srv ExternalSubscriberServer) {
	s.RegisterService(&ExternalSubscriber_ServiceDesc, srv)
}

func _ExternalSubscriber_ProcessBlock_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BlockRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ExternalSubscriberServer).ProcessBlock(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/subscriber.ExternalSubscriber/ProcessBlock",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ExternalSubscriberServer).ProcessBlock(ctx, req.(*BlockRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ExternalSubscriber_Initialize_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(InitRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ExternalSubscriberServer).Initialize(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/subscriber.ExternalSubscriber/Initialize",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ExternalSubscriberServer).Initialize(ctx, req.(*InitRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ExternalSubscriber_ServiceDesc is the grpc.ServiceDesc for ExternalSubscriber service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ExternalSubscriber_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "subscriber.ExternalSubscriber",
	HandlerType: (*ExternalSubscriberServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ProcessBlock",
			Handler:    _ExternalSubscriber_ProcessBlock_Handler,
		},
		{
			MethodName: "Initialize",
			Handler:    _ExternalSubscriber_Initialize_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "subscriber.proto",
}
