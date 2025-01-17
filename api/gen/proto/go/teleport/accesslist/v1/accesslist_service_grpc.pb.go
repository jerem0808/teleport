// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: teleport/accesslist/v1/accesslist_service.proto

package accesslistv1

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

const (
	AccessListService_GetAccessLists_FullMethodName       = "/teleport.accesslist.v1.AccessListService/GetAccessLists"
	AccessListService_ListAccessLists_FullMethodName      = "/teleport.accesslist.v1.AccessListService/ListAccessLists"
	AccessListService_GetAccessList_FullMethodName        = "/teleport.accesslist.v1.AccessListService/GetAccessList"
	AccessListService_UpsertAccessList_FullMethodName     = "/teleport.accesslist.v1.AccessListService/UpsertAccessList"
	AccessListService_DeleteAccessList_FullMethodName     = "/teleport.accesslist.v1.AccessListService/DeleteAccessList"
	AccessListService_DeleteAllAccessLists_FullMethodName = "/teleport.accesslist.v1.AccessListService/DeleteAllAccessLists"
)

// AccessListServiceClient is the client API for AccessListService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type AccessListServiceClient interface {
	// GetAccessLists returns a list of all access lists.
	GetAccessLists(ctx context.Context, in *GetAccessListsRequest, opts ...grpc.CallOption) (*GetAccessListsResponse, error)
	// ListAccessLists returns a paginated list of all access lists.
	ListAccessLists(ctx context.Context, in *ListAccessListsRequest, opts ...grpc.CallOption) (*ListAccessListsResponse, error)
	// GetAccessList returns the specified access list resource.
	GetAccessList(ctx context.Context, in *GetAccessListRequest, opts ...grpc.CallOption) (*AccessList, error)
	// UpsertAccessList creates or updates an access list resource.
	UpsertAccessList(ctx context.Context, in *UpsertAccessListRequest, opts ...grpc.CallOption) (*AccessList, error)
	// DeleteAccessList hard deletes the specified access list resource.
	DeleteAccessList(ctx context.Context, in *DeleteAccessListRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// DeleteAllAccessLists hard deletes all access lists.
	DeleteAllAccessLists(ctx context.Context, in *DeleteAllAccessListsRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type accessListServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewAccessListServiceClient(cc grpc.ClientConnInterface) AccessListServiceClient {
	return &accessListServiceClient{cc}
}

func (c *accessListServiceClient) GetAccessLists(ctx context.Context, in *GetAccessListsRequest, opts ...grpc.CallOption) (*GetAccessListsResponse, error) {
	out := new(GetAccessListsResponse)
	err := c.cc.Invoke(ctx, AccessListService_GetAccessLists_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *accessListServiceClient) ListAccessLists(ctx context.Context, in *ListAccessListsRequest, opts ...grpc.CallOption) (*ListAccessListsResponse, error) {
	out := new(ListAccessListsResponse)
	err := c.cc.Invoke(ctx, AccessListService_ListAccessLists_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *accessListServiceClient) GetAccessList(ctx context.Context, in *GetAccessListRequest, opts ...grpc.CallOption) (*AccessList, error) {
	out := new(AccessList)
	err := c.cc.Invoke(ctx, AccessListService_GetAccessList_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *accessListServiceClient) UpsertAccessList(ctx context.Context, in *UpsertAccessListRequest, opts ...grpc.CallOption) (*AccessList, error) {
	out := new(AccessList)
	err := c.cc.Invoke(ctx, AccessListService_UpsertAccessList_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *accessListServiceClient) DeleteAccessList(ctx context.Context, in *DeleteAccessListRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, AccessListService_DeleteAccessList_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *accessListServiceClient) DeleteAllAccessLists(ctx context.Context, in *DeleteAllAccessListsRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, AccessListService_DeleteAllAccessLists_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AccessListServiceServer is the server API for AccessListService service.
// All implementations must embed UnimplementedAccessListServiceServer
// for forward compatibility
type AccessListServiceServer interface {
	// GetAccessLists returns a list of all access lists.
	GetAccessLists(context.Context, *GetAccessListsRequest) (*GetAccessListsResponse, error)
	// ListAccessLists returns a paginated list of all access lists.
	ListAccessLists(context.Context, *ListAccessListsRequest) (*ListAccessListsResponse, error)
	// GetAccessList returns the specified access list resource.
	GetAccessList(context.Context, *GetAccessListRequest) (*AccessList, error)
	// UpsertAccessList creates or updates an access list resource.
	UpsertAccessList(context.Context, *UpsertAccessListRequest) (*AccessList, error)
	// DeleteAccessList hard deletes the specified access list resource.
	DeleteAccessList(context.Context, *DeleteAccessListRequest) (*emptypb.Empty, error)
	// DeleteAllAccessLists hard deletes all access lists.
	DeleteAllAccessLists(context.Context, *DeleteAllAccessListsRequest) (*emptypb.Empty, error)
	mustEmbedUnimplementedAccessListServiceServer()
}

// UnimplementedAccessListServiceServer must be embedded to have forward compatible implementations.
type UnimplementedAccessListServiceServer struct {
}

func (UnimplementedAccessListServiceServer) GetAccessLists(context.Context, *GetAccessListsRequest) (*GetAccessListsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetAccessLists not implemented")
}
func (UnimplementedAccessListServiceServer) ListAccessLists(context.Context, *ListAccessListsRequest) (*ListAccessListsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListAccessLists not implemented")
}
func (UnimplementedAccessListServiceServer) GetAccessList(context.Context, *GetAccessListRequest) (*AccessList, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetAccessList not implemented")
}
func (UnimplementedAccessListServiceServer) UpsertAccessList(context.Context, *UpsertAccessListRequest) (*AccessList, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpsertAccessList not implemented")
}
func (UnimplementedAccessListServiceServer) DeleteAccessList(context.Context, *DeleteAccessListRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteAccessList not implemented")
}
func (UnimplementedAccessListServiceServer) DeleteAllAccessLists(context.Context, *DeleteAllAccessListsRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteAllAccessLists not implemented")
}
func (UnimplementedAccessListServiceServer) mustEmbedUnimplementedAccessListServiceServer() {}

// UnsafeAccessListServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to AccessListServiceServer will
// result in compilation errors.
type UnsafeAccessListServiceServer interface {
	mustEmbedUnimplementedAccessListServiceServer()
}

func RegisterAccessListServiceServer(s grpc.ServiceRegistrar, srv AccessListServiceServer) {
	s.RegisterService(&AccessListService_ServiceDesc, srv)
}

func _AccessListService_GetAccessLists_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetAccessListsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AccessListServiceServer).GetAccessLists(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AccessListService_GetAccessLists_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AccessListServiceServer).GetAccessLists(ctx, req.(*GetAccessListsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AccessListService_ListAccessLists_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListAccessListsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AccessListServiceServer).ListAccessLists(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AccessListService_ListAccessLists_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AccessListServiceServer).ListAccessLists(ctx, req.(*ListAccessListsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AccessListService_GetAccessList_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetAccessListRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AccessListServiceServer).GetAccessList(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AccessListService_GetAccessList_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AccessListServiceServer).GetAccessList(ctx, req.(*GetAccessListRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AccessListService_UpsertAccessList_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpsertAccessListRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AccessListServiceServer).UpsertAccessList(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AccessListService_UpsertAccessList_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AccessListServiceServer).UpsertAccessList(ctx, req.(*UpsertAccessListRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AccessListService_DeleteAccessList_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteAccessListRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AccessListServiceServer).DeleteAccessList(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AccessListService_DeleteAccessList_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AccessListServiceServer).DeleteAccessList(ctx, req.(*DeleteAccessListRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AccessListService_DeleteAllAccessLists_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteAllAccessListsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AccessListServiceServer).DeleteAllAccessLists(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AccessListService_DeleteAllAccessLists_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AccessListServiceServer).DeleteAllAccessLists(ctx, req.(*DeleteAllAccessListsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// AccessListService_ServiceDesc is the grpc.ServiceDesc for AccessListService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var AccessListService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "teleport.accesslist.v1.AccessListService",
	HandlerType: (*AccessListServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetAccessLists",
			Handler:    _AccessListService_GetAccessLists_Handler,
		},
		{
			MethodName: "ListAccessLists",
			Handler:    _AccessListService_ListAccessLists_Handler,
		},
		{
			MethodName: "GetAccessList",
			Handler:    _AccessListService_GetAccessList_Handler,
		},
		{
			MethodName: "UpsertAccessList",
			Handler:    _AccessListService_UpsertAccessList_Handler,
		},
		{
			MethodName: "DeleteAccessList",
			Handler:    _AccessListService_DeleteAccessList_Handler,
		},
		{
			MethodName: "DeleteAllAccessLists",
			Handler:    _AccessListService_DeleteAllAccessLists_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "teleport/accesslist/v1/accesslist_service.proto",
}
