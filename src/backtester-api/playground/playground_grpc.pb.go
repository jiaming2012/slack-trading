// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.28.2
// source: playground.proto

package playground

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	PlaygroundService_CreatePlayground_FullMethodName = "/playground.PlaygroundService/CreatePlayground"
	PlaygroundService_NextTick_FullMethodName         = "/playground.PlaygroundService/NextTick"
	PlaygroundService_PlaceOrder_FullMethodName       = "/playground.PlaygroundService/PlaceOrder"
	PlaygroundService_GetAccount_FullMethodName       = "/playground.PlaygroundService/GetAccount"
	PlaygroundService_GetCandles_FullMethodName       = "/playground.PlaygroundService/GetCandles"
)

// PlaygroundServiceClient is the client API for PlaygroundService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type PlaygroundServiceClient interface {
	CreatePlayground(ctx context.Context, in *CreatePolygonPlaygroundRequest, opts ...grpc.CallOption) (*CreatePlaygroundResponse, error)
	NextTick(ctx context.Context, in *NextTickRequest, opts ...grpc.CallOption) (*TickDelta, error)
	PlaceOrder(ctx context.Context, in *PlaceOrderRequest, opts ...grpc.CallOption) (*Order, error)
	GetAccount(ctx context.Context, in *GetAccountRequest, opts ...grpc.CallOption) (*GetAccountResponse, error)
	GetCandles(ctx context.Context, in *GetCandlesRequest, opts ...grpc.CallOption) (*GetCandlesResponse, error)
}

type playgroundServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewPlaygroundServiceClient(cc grpc.ClientConnInterface) PlaygroundServiceClient {
	return &playgroundServiceClient{cc}
}

func (c *playgroundServiceClient) CreatePlayground(ctx context.Context, in *CreatePolygonPlaygroundRequest, opts ...grpc.CallOption) (*CreatePlaygroundResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CreatePlaygroundResponse)
	err := c.cc.Invoke(ctx, PlaygroundService_CreatePlayground_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *playgroundServiceClient) NextTick(ctx context.Context, in *NextTickRequest, opts ...grpc.CallOption) (*TickDelta, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(TickDelta)
	err := c.cc.Invoke(ctx, PlaygroundService_NextTick_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *playgroundServiceClient) PlaceOrder(ctx context.Context, in *PlaceOrderRequest, opts ...grpc.CallOption) (*Order, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Order)
	err := c.cc.Invoke(ctx, PlaygroundService_PlaceOrder_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *playgroundServiceClient) GetAccount(ctx context.Context, in *GetAccountRequest, opts ...grpc.CallOption) (*GetAccountResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetAccountResponse)
	err := c.cc.Invoke(ctx, PlaygroundService_GetAccount_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *playgroundServiceClient) GetCandles(ctx context.Context, in *GetCandlesRequest, opts ...grpc.CallOption) (*GetCandlesResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetCandlesResponse)
	err := c.cc.Invoke(ctx, PlaygroundService_GetCandles_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PlaygroundServiceServer is the server API for PlaygroundService service.
// All implementations must embed UnimplementedPlaygroundServiceServer
// for forward compatibility.
type PlaygroundServiceServer interface {
	CreatePlayground(context.Context, *CreatePolygonPlaygroundRequest) (*CreatePlaygroundResponse, error)
	NextTick(context.Context, *NextTickRequest) (*TickDelta, error)
	PlaceOrder(context.Context, *PlaceOrderRequest) (*Order, error)
	GetAccount(context.Context, *GetAccountRequest) (*GetAccountResponse, error)
	GetCandles(context.Context, *GetCandlesRequest) (*GetCandlesResponse, error)
	mustEmbedUnimplementedPlaygroundServiceServer()
}

// UnimplementedPlaygroundServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedPlaygroundServiceServer struct{}

func (UnimplementedPlaygroundServiceServer) CreatePlayground(context.Context, *CreatePolygonPlaygroundRequest) (*CreatePlaygroundResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreatePlayground not implemented")
}
func (UnimplementedPlaygroundServiceServer) NextTick(context.Context, *NextTickRequest) (*TickDelta, error) {
	return nil, status.Errorf(codes.Unimplemented, "method NextTick not implemented")
}
func (UnimplementedPlaygroundServiceServer) PlaceOrder(context.Context, *PlaceOrderRequest) (*Order, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PlaceOrder not implemented")
}
func (UnimplementedPlaygroundServiceServer) GetAccount(context.Context, *GetAccountRequest) (*GetAccountResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetAccount not implemented")
}
func (UnimplementedPlaygroundServiceServer) GetCandles(context.Context, *GetCandlesRequest) (*GetCandlesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetCandles not implemented")
}
func (UnimplementedPlaygroundServiceServer) mustEmbedUnimplementedPlaygroundServiceServer() {}
func (UnimplementedPlaygroundServiceServer) testEmbeddedByValue()                           {}

// UnsafePlaygroundServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to PlaygroundServiceServer will
// result in compilation errors.
type UnsafePlaygroundServiceServer interface {
	mustEmbedUnimplementedPlaygroundServiceServer()
}

func RegisterPlaygroundServiceServer(s grpc.ServiceRegistrar, srv PlaygroundServiceServer) {
	// If the following call pancis, it indicates UnimplementedPlaygroundServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&PlaygroundService_ServiceDesc, srv)
}

func _PlaygroundService_CreatePlayground_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreatePolygonPlaygroundRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PlaygroundServiceServer).CreatePlayground(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PlaygroundService_CreatePlayground_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PlaygroundServiceServer).CreatePlayground(ctx, req.(*CreatePolygonPlaygroundRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PlaygroundService_NextTick_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NextTickRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PlaygroundServiceServer).NextTick(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PlaygroundService_NextTick_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PlaygroundServiceServer).NextTick(ctx, req.(*NextTickRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PlaygroundService_PlaceOrder_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PlaceOrderRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PlaygroundServiceServer).PlaceOrder(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PlaygroundService_PlaceOrder_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PlaygroundServiceServer).PlaceOrder(ctx, req.(*PlaceOrderRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PlaygroundService_GetAccount_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetAccountRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PlaygroundServiceServer).GetAccount(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PlaygroundService_GetAccount_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PlaygroundServiceServer).GetAccount(ctx, req.(*GetAccountRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PlaygroundService_GetCandles_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetCandlesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PlaygroundServiceServer).GetCandles(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PlaygroundService_GetCandles_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PlaygroundServiceServer).GetCandles(ctx, req.(*GetCandlesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// PlaygroundService_ServiceDesc is the grpc.ServiceDesc for PlaygroundService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var PlaygroundService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "playground.PlaygroundService",
	HandlerType: (*PlaygroundServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreatePlayground",
			Handler:    _PlaygroundService_CreatePlayground_Handler,
		},
		{
			MethodName: "NextTick",
			Handler:    _PlaygroundService_NextTick_Handler,
		},
		{
			MethodName: "PlaceOrder",
			Handler:    _PlaygroundService_PlaceOrder_Handler,
		},
		{
			MethodName: "GetAccount",
			Handler:    _PlaygroundService_GetAccount_Handler,
		},
		{
			MethodName: "GetCandles",
			Handler:    _PlaygroundService_GetCandles_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "playground.proto",
}