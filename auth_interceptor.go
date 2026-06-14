package main

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// authContextKey is the key used to store auth info in gRPC context
type authContextKey string

const (
	userIDKey   authContextKey = "user_id"
	usernameKey authContextKey = "username"
	deviceIDKey authContextKey = "device_id"
)

// AuthInterceptor validates JWT Bearer tokens on every gRPC call (except AuthService).
// On success it injects user_id, username, device_id into the context.
func AuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Skip auth for AuthService — sign in/up/refresh don't have tokens yet
	if strings.HasPrefix(info.FullMethod, "/messenger.AuthService/") {
		return handler(ctx, req)
	}

	claims, err := extractAndValidateToken(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthenticated: %v", err)
	}

	// Inject auth info into context
	ctx = context.WithValue(ctx, userIDKey, claims.UserID)
	ctx = context.WithValue(ctx, usernameKey, claims.Username)
	ctx = context.WithValue(ctx, deviceIDKey, claims.DeviceID)

	return handler(ctx, req)
}

// AuthStreamInterceptor is the streaming variant of AuthInterceptor
func AuthStreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	// Skip auth for AuthService
	if strings.HasPrefix(info.FullMethod, "/messenger.AuthService/") {
		return handler(srv, ss)
	}

	// Legacy streams (v1 clients without JWT): Chat, Typing, CallSession
	// These streams handle auth internally (password in first message / username-based)
	switch info.FullMethod {
	case "/messenger.ChatService/Chat",
		"/messenger.ChatService/Typing",
		"/messenger.ChatService/CallSession":
		return handler(srv, ss)
	}

	claims, err := extractAndValidateToken(ss.Context())
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "unauthenticated: %v", err)
	}

	// Wrap the stream to inject auth context
 wrapped := &authServerStream{
		ServerStream:     ss,
		ctx: context.WithValue(ss.Context(), userIDKey, claims.UserID),
	}
	_ = claims
	return handler(srv, wrapped)
}

// authServerStream wraps grpc.ServerStream with modified context
type authServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authServerStream) Context() context.Context {
	return s.ctx
}

// GetUserID extracts user_id from context (set by interceptor)
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	return ""
}

// GetUsername extracts username from context (set by interceptor)
func GetUsername(ctx context.Context) string {
	if v, ok := ctx.Value(usernameKey).(string); ok {
		return v
	}
	return ""
}

// GetDeviceID extracts device_id from context (set by interceptor)
func GetDeviceID(ctx context.Context) string {
	if v, ok := ctx.Value(deviceIDKey).(string); ok {
		return v
	}
	return ""
}

// extractAndValidateToken reads the Bearer token from gRPC metadata and validates it
func extractAndValidateToken(ctx context.Context) (*authClaims, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	authHeader := values[0]
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization format")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := ValidateToken(token)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	if claims.Type != "access" {
		return nil, status.Error(codes.Unauthenticated, "expected access token, got refresh token")
	}

	return claims, nil
}
