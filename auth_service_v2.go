package main

import (
	"LavenderMessenger/gen"
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// authServerV2 implements the V2 AuthService methods with JWT + device management
// It wraps the existing authServer for v1 methods and adds v2 methods
type authServerV2 struct {
	*authServer
}

func newAuthServerV2(db *DB) *authServerV2 {
	return &authServerV2{authServer: newAuthServer(db)}
}

// SignInV2 authenticates a user with username/password, registers the device,
// and returns JWT access + refresh tokens
func (a *authServerV2) SignInV2(ctx context.Context, req *gen.SignInRequestV2) (*gen.AuthResponseV2, error) {
	username := req.GetUsername()
	password := req.GetPassword()
	deviceInfo := req.GetDevice()

	if username == "" || password == "" {
		return &gen.AuthResponseV2{
			Success: false,
			Message: "username and password are required",
		}, nil
	}

	// Validate credentials (reuse v1 logic)
	exists, err := a.db.UserExists(username)
	if err != nil {
		logger.Errorf("SignInV2: UserExists error for %s: %v", username, err)
		a.db.LogAuthEvent("", "", "signin_v2", "", req.GetClientVersion(), false, "internal error")
		return &gen.AuthResponseV2{
			Success: false,
			Message: "internal error",
		}, nil
	}

	if !exists {
		a.db.LogAuthEvent("", "", "signin_v2", "", req.GetClientVersion(), false, "user not found")
		return &gen.AuthResponseV2{
			Success: false,
			Message: "user not found",
		}, nil
	}

	storedHash, err := a.db.GetUserPasswordHash(username)
	if err != nil {
		logger.Errorf("SignInV2: GetUserPasswordHash error for %s: %v", username, err)
		return &gen.AuthResponseV2{
			Success: false,
			Message: "internal error",
		}, nil
	}

	if !CheckPassword(password, storedHash) {
		a.db.LogAuthEvent("", "", "signin_v2", "", req.GetClientVersion(), false, "invalid password")
		return &gen.AuthResponseV2{
			Success: false,
			Message: "invalid password",
		}, nil
	}

	// Get user data
	userID, _ := a.db.GetUserIdByUsername(username)
	email, bio, status, createdAt, lastSeenAt, _ := a.db.queryUserProfile(username)
	avatarURL, _ := a.db.GetUserAvatar(username)

	// Extract device info
	deviceID := ""
	deviceName := ""
	deviceType := "unknown"
	if deviceInfo != nil {
		deviceID = deviceInfo.GetDeviceId()
		deviceName = deviceInfo.GetDeviceName()
	}
	if deviceID == "" {
		deviceID = userID + "_unknown"
	}

	// Determine device type from client version string (rough heuristic)
	clientVersion := req.GetClientVersion()

	// Register/update device
	ipAddress := "unknown" // TODO: extract from gRPC context if needed
	userAgent := clientVersion
	_, err = a.db.UpsertDevice(userID, deviceID, deviceName, deviceType, clientVersion, ipAddress, userAgent)
	if err != nil {
		logger.Errorf("SignInV2: UpsertDevice error for %s/%s: %v", username, deviceID, err)
	}

	// Generate JWT token pair
	accessToken, refreshToken, accessExp, refreshExp, err := GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		logger.Errorf("SignInV2: GenerateTokenPair error for %s: %v", username, err)
		return &gen.AuthResponseV2{
			Success: false,
			Message: "failed to generate tokens",
		}, nil
	}

	// Store refresh token JTI in DB for validation on refresh
	refreshJTI, _ := ExtractJTI(refreshToken)
	if refreshJTI != "" {
		a.db.UpdateDeviceRefreshToken(userID, deviceID, refreshJTI, refreshExp)
	}

	// Update last seen on user
	_ = a.db.UpdateLastSeen(username)

	// Log success
	a.db.LogAuthEvent(userID, deviceID, "signin_v2", ipAddress, clientVersion, true, "")

	logger.Infof("SignInV2: %s (ID: %s, device: %s)", username, userID, deviceID)

	return &gen.AuthResponseV2{
		Success:         true,
		Message:         "sign in successful",
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		AccessExpiresAt: accessExp.Unix(),
		RefreshExpiresAt: refreshExp.Unix(),
		User: &gen.User{
			Id:         userID,
			Username:   username,
			Email:      email,
			AvatarUrl:  avatarURL,
			Bio:        bio,
			Status:     status,
			CreatedAt:  timestamppb.New(createdAt),
			LastSeenAt: timestamppb.New(lastSeenAt),
		},
	}, nil
}

// SignUpV2 registers a new user, registers the device, and returns JWT tokens
func (a *authServerV2) SignUpV2(ctx context.Context, req *gen.SignUpRequestV2) (*gen.AuthResponseV2, error) {
	username := req.GetUsername()
	password := req.GetPassword()
	email := req.GetEmail()
	deviceInfo := req.GetDevice()

	if username == "" || password == "" {
		return &gen.AuthResponseV2{
			Success: false,
			Message: "username and password are required",
		}, nil
	}

	exists, err := a.db.UserExists(username)
	if err != nil {
		logger.Errorf("SignUpV2: UserExists error for %s: %v", username, err)
		return &gen.AuthResponseV2{
			Success: false,
			Message: "internal error",
		}, nil
	}

	if exists {
		return &gen.AuthResponseV2{
			Success: false,
			Message: "username already taken",
		}, nil
	}

	if email != "" {
		emailExists, err := a.db.EmailExists(email)
		if err != nil {
			logger.Errorf("SignUpV2: EmailExists error for %s: %v", email, err)
			return &gen.AuthResponseV2{
				Success: false,
				Message: "internal error",
			}, nil
		}
		if emailExists {
			return &gen.AuthResponseV2{
				Success: false,
				Message: "email already in use",
			}, nil
		}
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		logger.Errorf("SignUpV2: HashPassword error for %s: %v", username, err)
		return &gen.AuthResponseV2{
			Success: false,
			Message: "internal error",
		}, nil
	}

	err = a.db.SaveUserWithEmail(username, passwordHash, email)
	if err != nil {
		logger.Errorf("SignUpV2: SaveUserWithEmail error for %s: %v", username, err)
		return &gen.AuthResponseV2{
			Success: false,
			Message: fmt.Sprintf("failed to create user: %v", err),
		}, nil
	}

	userID, _ := a.db.GetUserIdByUsername(username)

	// Device info
	deviceID := ""
	deviceName := ""
	deviceType := "unknown"
	if deviceInfo != nil {
		deviceID = deviceInfo.GetDeviceId()
		deviceName = deviceInfo.GetDeviceName()
	}
	if deviceID == "" {
		deviceID = userID + "_unknown"
	}

	clientVersion := req.GetClientVersion()

	// Register device
	_, err = a.db.UpsertDevice(userID, deviceID, deviceName, deviceType, clientVersion, "unknown", clientVersion)
	if err != nil {
		logger.Errorf("SignUpV2: UpsertDevice error for %s/%s: %v", username, deviceID, err)
	}

	// Generate JWT token pair
	accessToken, refreshToken, accessExp, refreshExp, err := GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		logger.Errorf("SignUpV2: GenerateTokenPair error for %s: %v", username, err)
		return &gen.AuthResponseV2{
			Success: false,
			Message: "failed to generate tokens",
		}, nil
	}

	// Store refresh token JTI
	refreshJTI, _ := ExtractJTI(refreshToken)
	if refreshJTI != "" {
		a.db.UpdateDeviceRefreshToken(userID, deviceID, refreshJTI, refreshExp)
	}

	a.db.LogAuthEvent(userID, deviceID, "signup_v2", "unknown", clientVersion, true, "")

	logger.Infof("SignUpV2: new user %s (ID: %s, device: %s)", username, userID, deviceID)

	return &gen.AuthResponseV2{
		Success:         true,
		Message:         "sign up successful",
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		AccessExpiresAt: accessExp.Unix(),
		RefreshExpiresAt: refreshExp.Unix(),
		User: &gen.User{
			Id:        userID,
			Username:  username,
			Email:     email,
		},
	}, nil
}

// RefreshToken takes a refresh token, validates it, and issues a new access+refresh pair
func (a *authServerV2) RefreshToken(ctx context.Context, req *gen.RefreshTokenRequest) (*gen.RefreshTokenResponse, error) {
	refreshTokenStr := req.GetRefreshToken()

	// Validate the refresh token JWT
	claims, err := ValidateToken(refreshTokenStr)
	if err != nil {
		return &gen.RefreshTokenResponse{}, fmt.Errorf("invalid refresh token: %w", err)
	}

	if claims.Type != "refresh" {
		return &gen.RefreshTokenResponse{}, fmt.Errorf("expected refresh token")
	}

	// Check that the JTI matches what's stored in DB (token rotation)
	valid, err := a.db.ValidateRefreshToken(claims.UserID, claims.DeviceID, claims.ID)
	if err != nil || !valid {
		// Token reuse detected or expired — revoke all tokens for this device
		_ = a.db.RevokeDevice(claims.UserID, claims.DeviceID)
		a.db.LogAuthEvent(claims.UserID, claims.DeviceID, "refresh_reuse_detected", "", "", false, "token reuse or expired")
		return &gen.RefreshTokenResponse{}, fmt.Errorf("refresh token revoked or expired")
	}

	username := claims.Username

	// Generate new token pair (rotation: new refresh token)
	accessToken, refreshToken, accessExp, refreshExp, err := GenerateTokenPair(claims.UserID, username, claims.DeviceID)
	if err != nil {
		logger.Errorf("RefreshToken: GenerateTokenPair error for %s: %v", username, err)
		return &gen.RefreshTokenResponse{}, fmt.Errorf("failed to generate tokens")
	}

	// Store new refresh token JTI
	newJTI, _ := ExtractJTI(refreshToken)
	if newJTI != "" {
		a.db.UpdateDeviceRefreshToken(claims.UserID, claims.DeviceID, newJTI, refreshExp)
	}

	a.db.LogAuthEvent(claims.UserID, claims.DeviceID, "refresh", "", "", true, "")

	return &gen.RefreshTokenResponse{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresAt:  accessExp.Unix(),
		RefreshExpiresAt: refreshExp.Unix(),
	}, nil
}

// SignOut revokes a specific device session or all devices
func (a *authServerV2) SignOut(ctx context.Context, req *gen.SignOutRequest) (*gen.AuthResponse, error) {
	// If we have a refresh token, extract user/device from it
	var userID, deviceID string

	if req.GetRefreshToken() != "" {
		claims, err := ValidateToken(req.GetRefreshToken())
		if err == nil {
			userID = claims.UserID
			deviceID = claims.DeviceID
		}
	}

	if req.GetAllDevices() && userID != "" {
		err := a.db.RevokeAllDevices(userID)
		if err != nil {
			logger.Errorf("SignOut: RevokeAllDevices error for %s: %v", userID, err)
		}
		a.db.LogAuthEvent(userID, "", "signout_all", "", "", true, "")
		logger.Infof("SignOut: all devices revoked for user %s", userID)
	} else if userID != "" && deviceID != "" {
		err := a.db.RevokeDevice(userID, deviceID)
		if err != nil {
			logger.Errorf("SignOut: RevokeDevice error for %s/%s: %v", userID, deviceID, err)
		}
		a.db.LogAuthEvent(userID, deviceID, "signout", "", "", true, "")
		logger.Infof("SignOut: device %s revoked for user %s", deviceID, userID)
	}

	return &gen.AuthResponse{
		Success: true,
		Message: "signed out",
	}, nil
}

// RevokeDevice deactivates a specific device
func (a *authServerV2) RevokeDevice(ctx context.Context, req *gen.RevokeDeviceRequest) (*gen.AuthResponse, error) {
	deviceID := req.GetDeviceId()
	fromContext := GetUserID(ctx)
	if fromContext == "" {
		return &gen.AuthResponse{Success: false, Message: "unauthenticated"}, nil
	}

	err := a.db.RevokeDevice(fromContext, deviceID)
	if err != nil {
		return &gen.AuthResponse{Success: false, Message: "failed to revoke device"}, nil
	}

	a.db.LogAuthEvent(fromContext, deviceID, "revoke", "", "", true, "")
	logger.Infof("RevokeDevice: device %s revoked for user %s", deviceID, fromContext)

	return &gen.AuthResponse{Success: true, Message: "device revoked"}, nil
}

// GetDevices returns all active devices for the authenticated user
func (a *authServerV2) GetDevices(ctx context.Context, req *gen.GetDevicesRequest) (*gen.GetDevicesResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.GetDevicesResponse{}, nil
	}

	devices, err := a.db.GetDevices(userID)
	if err != nil {
		logger.Errorf("GetDevices: error for %s: %v", userID, err)
		return &gen.GetDevicesResponse{}, nil
	}

	var result []*gen.DeviceInfo
	for _, d := range devices {
		result = append(result, &gen.DeviceInfo{
			DeviceId:      d.DeviceID,
			DeviceName:    d.DeviceName,
			ClientVersion: d.ClientVersion,
			LastSeenAt:    timestamppb.New(d.LastSeenAt),
			IpAddress:     d.IPAddress,
		})
	}

	return &gen.GetDevicesResponse{Devices: result}, nil
}
