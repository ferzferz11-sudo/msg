package main

import (
	"LavenderMessenger/gen"
	"context"
)

func (s *server) GetThemes(ctx context.Context, req *gen.GetThemesRequest) (*gen.GetThemesResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	var currentID string
	var themes []struct {
		ThemeID, Name, PrimaryColor, OnPrimaryColor, SurfaceColor, OnSurfaceColor, BackgroundColor, TextPrimaryColor, TextSecondaryColor                     string
		IsDark                                                                                                                                               bool
		ChatBackgroundImageUrl, ChatListBackgroundImageUrl, BottomPanelColor, OnBottomPanelColor, SurfaceContainer, OutgoingBubbleColor, IncomingBubbleColor string
	}
	var err error

	if isUUID(userID) {
		currentID, themes, err = s.db.GetUserThemesByUserID(userID)
	} else {
		currentID, themes, err = s.db.GetUserThemes(userID)
	}

	if err != nil {
		s.logErrorOnce("GetThemes:"+userID, "Failed to get themes for %s: %v", userID, err)
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

	logger.Infof("Retrieved %d custom themes for user %s (Current: %s)", len(customThemes), userID, currentID)

	return &gen.GetThemesResponse{
		CurrentThemeId: currentID,
		CustomThemes:   customThemes,
	}, nil
}

func (s *server) SaveTheme(ctx context.Context, req *gen.SaveThemeRequest) (*gen.SaveThemeResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	logger.Infof("Saving theme '%s' (ID: %s) for user %s. Chat Background URL: %s", req.Theme.Name, req.Theme.Id, userID, req.Theme.ChatBackgroundImageUrl)

	var err error
	if isUUID(userID) {
		err = s.db.SaveUserThemeByUserID(userID, req.Theme)
	} else {
		err = s.db.SaveUserTheme(userID, req.Theme)
	}

	if err != nil {
		s.logErrorOnce("SaveTheme:"+userID, "Failed to save theme for %s: %v", userID, err)
		return &gen.SaveThemeResponse{Success: false, Message: err.Error()}, nil
	}
	logger.Infof("Theme '%s' saved successfully for %s", req.Theme.Name, userID)
	return &gen.SaveThemeResponse{Success: true, Message: "Theme saved"}, nil
}

func (s *server) SetCurrentTheme(ctx context.Context, req *gen.SetCurrentThemeRequest) (*gen.SetCurrentThemeResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	logger.Infof("Setting current theme to %s for user %s", req.ThemeId, userID)

	var err error
	if isUUID(userID) {
		err = s.db.SetCurrentThemeByUserID(userID, req.ThemeId)
	} else {
		err = s.db.SetCurrentTheme(userID, req.ThemeId)
	}

	if err != nil {
		s.logErrorOnce("SetCurrentTheme:"+userID, "Failed to set current theme for %s: %v", userID, err)
		return &gen.SetCurrentThemeResponse{Success: false}, nil
	}
	return &gen.SetCurrentThemeResponse{Success: true}, nil
}

func (s *server) DeleteTheme(ctx context.Context, req *gen.DeleteThemeRequest) (*gen.DeleteThemeResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		userID = req.UserId
	}

	logger.Infof("Deleting theme %s for user %s", req.ThemeId, userID)

	var err error
	if isUUID(userID) {
		err = s.db.DeleteUserThemeByUserID(userID, req.ThemeId)
	} else {
		err = s.db.DeleteUserTheme(userID, req.ThemeId)
	}

	if err != nil {
		s.logErrorOnce("DeleteTheme:"+userID, "Failed to delete theme for %s: %v", userID, err)
		return &gen.DeleteThemeResponse{Success: false}, nil
	}
	logger.Infof("Theme %s deleted successfully for %s", req.ThemeId, userID)
	return &gen.DeleteThemeResponse{Success: true}, nil
}
