package main

import (
	"LavenderMessenger/gen"
	"context"
	"log"
)

func (s *server) GetThemes(_ context.Context, req *gen.GetThemesRequest) (*gen.GetThemesResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolved := s.resolveUsername(req.UserId)
		if resolved != "" {
			username = resolved
		}
	}

	currentID, themes, err := s.db.GetUserThemes(username)
	if err != nil {
		s.logErrorOnce("GetThemes:"+username, "Failed to get themes for %s: %v", username, err)
		return &gen.GetThemesResponse{CurrentThemeId: "dark"}, nil
	}

	var customThemes []*gen.CustomTheme
	for _, t := range themes {
		customThemes = append(customThemes, &gen.CustomTheme{
			Id:                         t.ThemeID,
			Name:                       t.Name,
			PrimaryColor:               t.PrimaryColor,
			OnPrimaryColor:             t.OnPrimaryColor,
			SurfaceColor:               t.SurfaceColor,
			OnSurfaceColor:             t.OnSurfaceColor,
			BackgroundColor:            t.BackgroundColor,
			TextPrimaryColor:           t.TextPrimaryColor,
			TextSecondaryColor:         t.TextSecondaryColor,
			IsDark:                     t.IsDark,
			ChatBackgroundImageUrl:     t.ChatBackgroundImageUrl,
			ChatListBackgroundImageUrl: t.ChatListBackgroundImageUrl,
			BottomPanelColor:           t.BottomPanelColor,
			OnBottomPanelColor:         t.OnBottomPanelColor,
			SurfaceContainer:           t.SurfaceContainer,
			OutgoingBubbleColor:        t.OutgoingBubbleColor,
			IncomingBubbleColor:        t.IncomingBubbleColor,
		})
	}

	log.Printf("Retrieved %d custom themes for user %s (Current: %s)", len(customThemes), username, currentID)

	return &gen.GetThemesResponse{
		CurrentThemeId: currentID,
		CustomThemes:   customThemes,
	}, nil
}

func (s *server) SaveTheme(_ context.Context, req *gen.SaveThemeRequest) (*gen.SaveThemeResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolved := s.resolveUsername(req.UserId)
		if resolved != "" {
			username = resolved
		}
	}

	log.Printf("Saving theme '%s' (ID: %s) for user %s. Chat Background URL: %s", req.Theme.Name, req.Theme.Id, username, req.Theme.ChatBackgroundImageUrl)
	err := s.db.SaveUserTheme(username, req.Theme)
	if err != nil {
		s.logErrorOnce("SaveTheme:"+username, "Failed to save theme for %s: %v", username, err)
		return &gen.SaveThemeResponse{Success: false, Message: err.Error()}, nil
	}
	log.Printf("Theme '%s' saved successfully for %s", req.Theme.Name, username)
	return &gen.SaveThemeResponse{Success: true, Message: "Theme saved"}, nil
}

func (s *server) SetCurrentTheme(_ context.Context, req *gen.SetCurrentThemeRequest) (*gen.SetCurrentThemeResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolved := s.resolveUsername(req.UserId)
		if resolved != "" {
			username = resolved
		}
	}

	log.Printf("Setting current theme to %s for user %s", req.ThemeId, username)
	err := s.db.SetCurrentTheme(username, req.ThemeId)
	if err != nil {
		s.logErrorOnce("SetCurrentTheme:"+username, "Failed to set current theme for %s: %v", username, err)
		return &gen.SetCurrentThemeResponse{Success: false}, nil
	}
	return &gen.SetCurrentThemeResponse{Success: true}, nil
}

func (s *server) DeleteTheme(_ context.Context, req *gen.DeleteThemeRequest) (*gen.DeleteThemeResponse, error) {
	username := req.Username
	if req.UserId != "" {
		resolved := s.resolveUsername(req.UserId)
		if resolved != "" {
			username = resolved
		}
	}

	log.Printf("Deleting theme %s for user %s", req.ThemeId, username)
	err := s.db.DeleteUserTheme(username, req.ThemeId)
	if err != nil {
		s.logErrorOnce("DeleteTheme:"+username, "Failed to delete theme for %s: %v", username, err)
		return &gen.DeleteThemeResponse{Success: false}, nil
	}
	log.Printf("Theme %s deleted successfully for %s", req.ThemeId, username)
	return &gen.DeleteThemeResponse{Success: true}, nil
}
