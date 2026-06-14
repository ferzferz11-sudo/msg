package main

import (
	"LavenderMessenger/gen"
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// serverServiceServer implements gen.ServerServiceServer
type serverServiceServer struct {
	gen.UnimplementedServerServiceServer
	db *DB
}

func (s *serverServiceServer) ListServers(ctx context.Context, req *gen.ListServersRequest) (*gen.ListServersResponse, error) {
	servers, err := s.db.GetAllServers()
	if err != nil {
		logger.Infof("ListServers: %v", err)
		return nil, status.Error(codes.Internal, "failed to list servers")
	}

	var result []*gen.ServerInfo
	for _, srv := range servers {
		result = append(result, &gen.ServerInfo{
			Id:        srv.ID,
			Name:      srv.Name,
			Host:      srv.Host,
			Port:      int32(srv.Port),
			IsDefault: srv.IsDefault,
		})
	}
	return &gen.ListServersResponse{Servers: result}, nil
}

func (s *serverServiceServer) GetDefaultServer(ctx context.Context, req *gen.GetDefaultServerRequest) (*gen.GetDefaultServerResponse, error) {
	srv, err := s.db.GetDefaultServer()
	if err != nil {
		return &gen.GetDefaultServerResponse{Success: false, Message: "no default server"}, nil
	}
	return &gen.GetDefaultServerResponse{
		Success: true,
		Server: &gen.ServerInfo{
			Id:   srv.ID,
			Name: srv.Name,
			Host: srv.Host,
			Port: int32(srv.Port),
		},
	}, nil
}

func (s *serverServiceServer) AddServer(ctx context.Context, req *gen.AddServerRequest) (*gen.AddServerResponse, error) {
	if req.Auth == nil || !s.db.IsSuperAdmin(req.Auth.AdminUserId) {
		return nil, status.Error(codes.PermissionDenied, "admin required")
	}
	id, err := s.db.CreateServer(req.Name, req.Host, int(req.Port), false)
	if err != nil {
		return &gen.AddServerResponse{Success: false, Message: err.Error()}, nil
	}
	return &gen.AddServerResponse{Success: true, ServerId: id}, nil
}

func (s *serverServiceServer) UpdateServer(ctx context.Context, req *gen.UpdateServerRequest) (*gen.UpdateServerResponse, error) {
	if req.Auth == nil || !s.db.IsSuperAdmin(req.Auth.AdminUserId) {
		return nil, status.Error(codes.PermissionDenied, "admin required")
	}
	err := s.db.UpdateServer(req.Id, req.Name, req.Host, int(req.Port))
	if err != nil {
		return &gen.UpdateServerResponse{Success: false, Message: err.Error()}, nil
	}
	return &gen.UpdateServerResponse{Success: true}, nil
}

func (s *serverServiceServer) DeleteServer(ctx context.Context, req *gen.DeleteServerRequest) (*gen.DeleteServerResponse, error) {
	if req.Auth == nil || !s.db.IsSuperAdmin(req.Auth.AdminUserId) {
		return nil, status.Error(codes.PermissionDenied, "admin required")
	}
	err := s.db.DeleteServer(req.Id)
	if err != nil {
		return &gen.DeleteServerResponse{Success: false, Message: err.Error()}, nil
	}
	return &gen.DeleteServerResponse{Success: true}, nil
}

func (s *serverServiceServer) SetDefaultServer(ctx context.Context, req *gen.SetDefaultServerRequest) (*gen.SetDefaultServerResponse, error) {
	if req.Auth == nil || !s.db.IsSuperAdmin(req.Auth.AdminUserId) {
		return nil, status.Error(codes.PermissionDenied, "admin required")
	}
	err := s.db.SetDefaultServer(req.Id)
	if err != nil {
		return &gen.SetDefaultServerResponse{Success: false, Message: err.Error()}, nil
	}
	return &gen.SetDefaultServerResponse{Success: true}, nil
}
