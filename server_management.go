package main

import (
	"context"
	"fmt"
	"log"

	"LavenderMessenger/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Ensure serverServiceServer is implemented
var _ gen.ServerServiceServer = (*serverServiceServer)(nil)

type serverServiceServer struct {
	gen.UnimplementedServerServiceServer
	db *DB
}

// ======= Public (no auth) =======

func (s *serverServiceServer) ListServers(ctx context.Context, req *gen.ListServersRequest) (*gen.ListServersResponse, error) {
	servers, err := s.db.GetAllServers()
	if err != nil {
		return &gen.ListServersResponse{}, err
	}

	var result []*gen.ServerInfo
	for _, srv := range servers {
		result = append(result, &gen.ServerInfo{
			Id:        srv.ID,
			Name:      srv.Name,
			Host:      srv.Host,
			Port:      int32(srv.Port),
			IsDefault: srv.IsDefault,
			CreatedAt: timestamppb.New(srv.CreatedAt),
		})
	}

	return &gen.ListServersResponse{Servers: result}, nil
}

func (s *serverServiceServer) GetDefaultServer(ctx context.Context, req *gen.GetDefaultServerRequest) (*gen.GetDefaultServerResponse, error) {
	srv, err := s.db.GetDefaultServer()
	if err != nil {
		return &gen.GetDefaultServerResponse{Success: false, Message: "No default server configured"}, nil
	}

	return &gen.GetDefaultServerResponse{
		Success: true,
		Message: "OK",
		Server: &gen.ServerInfo{
			Id:   srv.ID,
			Name: srv.Name,
			Host: srv.Host,
			Port: int32(srv.Port),
		},
	}, nil
}

// ======= Super Admin only =======

func (s *serverServiceServer) getAdminUsername(reqAdminUsername, reqAdminUserId string) string {
	if reqAdminUsername != "" {
		return reqAdminUsername
	}
	if reqAdminUserId != "" {
		resolved := resolveUsername(s.db, reqAdminUserId)
		if resolved != "" {
			return resolved
		}
	}
	return ""
}

func (s *serverServiceServer) checkSuperAdmin(reqAdminUsername, reqAdminUserId string) error {
	username := s.getAdminUsername(reqAdminUsername, reqAdminUserId)
	if username == "" {
		return fmt.Errorf("not authenticated")
	}
	if !s.db.IsSuperAdmin(username) {
		return fmt.Errorf("super admin required")
	}
	return nil
}

func (s *serverServiceServer) AddServer(ctx context.Context, req *gen.AddServerRequest) (*gen.AddServerResponse, error) {
	if err := s.checkSuperAdmin(req.GetAuth().GetAdminUsername(), req.GetAuth().GetAdminUserId()); err != nil {
		return &gen.AddServerResponse{Success: false, Message: err.Error()}, err
	}

	if req.Name == "" || req.Host == "" || req.Port == 0 {
		return &gen.AddServerResponse{Success: false, Message: "name, host and port are required"}, fmt.Errorf("invalid input")
	}

	id, err := s.db.CreateServer(req.Name, req.Host, int(req.Port), false)
	if err != nil {
		log.Printf("Failed to add server: %v", err)
		return &gen.AddServerResponse{Success: false, Message: "Failed to add server"}, err
	}

	log.Printf("Server added: %s (%s:%d) id=%s", req.Name, req.Host, req.Port, id)
	return &gen.AddServerResponse{Success: true, Message: "Server added", ServerId: id}, nil
}

func (s *serverServiceServer) UpdateServer(ctx context.Context, req *gen.UpdateServerRequest) (*gen.UpdateServerResponse, error) {
	if err := s.checkSuperAdmin(req.GetAuth().GetAdminUsername(), req.GetAuth().GetAdminUserId()); err != nil {
		return &gen.UpdateServerResponse{Success: false, Message: err.Error()}, err
	}

	if req.Id == "" || req.Name == "" || req.Host == "" || req.Port == 0 {
		return &gen.UpdateServerResponse{Success: false, Message: "id, name, host and port are required"}, fmt.Errorf("invalid input")
	}

	err := s.db.UpdateServer(req.Id, req.Name, req.Host, int(req.Port))
	if err != nil {
		log.Printf("Failed to update server: %v", err)
		return &gen.UpdateServerResponse{Success: false, Message: "Failed to update server"}, err
	}

	log.Printf("Server updated: %s (%s:%d) id=%s", req.Name, req.Host, req.Port, req.Id)
	return &gen.UpdateServerResponse{Success: true, Message: "Server updated"}, nil
}

func (s *serverServiceServer) DeleteServer(ctx context.Context, req *gen.DeleteServerRequest) (*gen.DeleteServerResponse, error) {
	if err := s.checkSuperAdmin(req.GetAuth().GetAdminUsername(), req.GetAuth().GetAdminUserId()); err != nil {
		return &gen.DeleteServerResponse{Success: false, Message: err.Error()}, err
	}

	if req.Id == "" {
		return &gen.DeleteServerResponse{Success: false, Message: "server id is required"}, fmt.Errorf("invalid input")
	}

	err := s.db.DeleteServer(req.Id)
	if err != nil {
		log.Printf("Failed to delete server: %v", err)
		return &gen.DeleteServerResponse{Success: false, Message: err.Error()}, err
	}

	log.Printf("Server deleted: %s", req.Id)
	return &gen.DeleteServerResponse{Success: true, Message: "Server deleted"}, nil
}

func (s *serverServiceServer) SetDefaultServer(ctx context.Context, req *gen.SetDefaultServerRequest) (*gen.SetDefaultServerResponse, error) {
	if err := s.checkSuperAdmin(req.GetAuth().GetAdminUsername(), req.GetAuth().GetAdminUserId()); err != nil {
		return &gen.SetDefaultServerResponse{Success: false, Message: err.Error()}, err
	}

	if req.Id == "" {
		return &gen.SetDefaultServerResponse{Success: false, Message: "server id is required"}, fmt.Errorf("invalid input")
	}

	err := s.db.SetDefaultServer(req.Id)
	if err != nil {
		log.Printf("Failed to set default server: %v", err)
		return &gen.SetDefaultServerResponse{Success: false, Message: "Failed to set default server"}, err
	}

	log.Printf("Default server set: %s", req.Id)
	return &gen.SetDefaultServerResponse{Success: true, Message: "Default server set"}, nil
}

// resolveUsername resolves user_id to username using DB
func resolveUsername(db *DB, userId string) string {
	var username string
	err := db.QueryRow(`SELECT id FROM users WHERE id=$1::uuid`, userId).Scan(&username)
	if err != nil {
		return ""
	}
	return username
}
