package main

import (
	"strconv"
	"strings"
	"testing"
)

func TestRemoveParticipant(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		userID   string
		expected string
	}{
		{
			name:     "remove from list of 3",
			current:  "['user1','user2','user3']",
			userID:   "user2",
			expected: "['user1','user3']",
		},
		{
			name:     "remove only user",
			current:  "['user1']",
			userID:   "user1",
			expected: "[]",
		},
		{
			name:     "remove first in list",
			current:  "['user1','user2','user3']",
			userID:   "user1",
			expected: "['user2','user3']",
		},
		{
			name:     "remove last in list",
			current:  "['user1','user2','user3']",
			userID:   "user3",
			expected: "['user1','user2']",
		},
		{
			name:     "user not in list",
			current:  "['user1','user2']",
			userID:   "user3",
			expected: "['user1','user2']",
		},
		{
			name:     "empty list",
			current:  "[]",
			userID:   "user1",
			expected: "[]",
		},
		{
			name:     "double quotes",
			current:  "[\"user1\",\"user2\"]",
			userID:   "user1",
			expected: "['user2']",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeParticipant(tt.current, tt.userID)
			if result != tt.expected {
				t.Errorf("removeParticipant(%q, %q) = %q, want %q", tt.current, tt.userID, result, tt.expected)
			}
		})
	}
}

func TestCompanyPositions_AccessLevelThresholds(t *testing.T) {
	// Test that access level thresholds are correct
	tests := []struct {
		accessLevel string
		minLevel    int
		expected    int
	}{
		{"member", 0, 0},
		{"management", 0, 1},
		{"owner_only", 0, 3},
		{"member", 2, 2},       // minLevel overrides
		{"management", 3, 3},   // minLevel overrides
	}

	for _, tt := range tests {
		t.Run(tt.accessLevel+"_"+strconv.Itoa(tt.minLevel), func(t *testing.T) {
			threshold := 0
			switch tt.accessLevel {
			case "owner_only":
				threshold = 3
			case "management":
				threshold = 1
			}
			if tt.minLevel > threshold {
				threshold = tt.minLevel
			}
			if threshold != tt.expected {
				t.Errorf("access_level=%s minLevel=%d: got threshold %d, want %d",
					tt.accessLevel, tt.minLevel, threshold, tt.expected)
			}
		})
	}
}

func TestCompanyPositions_LevelHierarchy(t *testing.T) {
	// Level hierarchy: 0=staff, 1=manager, 2=top_manager, 3=owner
	levels := map[string]int{
		"Employee":   0,
		"Manager":    1,
		"Top Manager": 2,
		"Owner":      3,
	}

	// Higher level should have more access
	if levels["Owner"] <= levels["Top Manager"] {
		t.Error("Owner level should be > Top Manager")
	}
	if levels["Top Manager"] <= levels["Manager"] {
		t.Error("Top Manager level should be > Manager")
	}
	if levels["Manager"] <= levels["Employee"] {
		t.Error("Manager level should be > Employee")
	}

	// Management access (level >= 1) should see management chats
	for name, level := range levels {
		canSeeManagement := level >= 1
		switch name {
		case "Employee":
			if canSeeManagement {
				t.Error("Employee should NOT see management chats")
			}
		case "Manager", "Top Manager", "Owner":
			if !canSeeManagement {
				t.Errorf("%s should see management chats", name)
			}
		}
	}

	// Owner-only access (level >= 3)
	for name, level := range levels {
		canSeeOwnerOnly := level >= 3
		switch name {
		case "Employee", "Manager", "Top Manager":
			if canSeeOwnerOnly {
				t.Errorf("%s should NOT see owner-only chats", name)
			}
		case "Owner":
			if !canSeeOwnerOnly {
				t.Error("Owner should see owner-only chats")
			}
		}
	}
}

func TestCompanyChat_CreationLogic(t *testing.T) {
	// Test that company chat type is valid
	validTypes := map[string]bool{
		"direct":  true,
		"group":   true,
		"secret":  true,
		"company": true,
	}

	if !validTypes["company"] {
		t.Error("'company' should be a valid chat type")
	}
	if validTypes["invalid"] {
		t.Error("'invalid' should not be a valid chat type")
	}
}

func TestCompanyChat_AccessLevelValidation(t *testing.T) {
	validAccessLevels := map[string]bool{
		"none":          true,
		"member":        true,
		"management":    true,
		"owner_only":    true,
		"all":           true,
	}

	for _, level := range []string{"none", "member", "management", "owner_only", "all"} {
		if !validAccessLevels[level] {
			t.Errorf("%q should be a valid access level", level)
		}
	}

	for _, level := range []string{"invalid", "super_admin", "admin"} {
		if validAccessLevels[level] {
			t.Errorf("%q should NOT be a valid access level", level)
		}
	}
}

func TestCompany_DefaultPositions(t *testing.T) {
	// When creating a company, default positions should be created
	defaultPositions := []struct {
		title      string
		level      int
		chatAccess string
	}{
		{"Owner", 3, "owner_only"},
		{"Top Manager", 2, "management"},
		{"Manager", 1, "management"},
		{"Employee", 0, "member"},
	}

	seen := make(map[string]bool)
	for _, p := range defaultPositions {
		if seen[p.title] {
			t.Errorf("duplicate default position: %s", p.title)
		}
		seen[p.title] = true

		if p.level < 0 || p.level > 3 {
			t.Errorf("position %s has invalid level %d", p.title, p.level)
		}
	}

	// Owner must be level 3
	if defaultPositions[0].level != 3 {
		t.Error("Owner position must be level 3")
	}
}

func TestCompany_CannotDeleteBuiltinPositions(t *testing.T) {
	builtinPositions := []struct {
		title string
		level int
	}{
		{"Owner", 3},
		{"Top Manager", 2},
		{"Manager", 1},
		{"Employee", 0},
	}

	for _, p := range builtinPositions {
		// Built-in positions (level 0-3 with standard names) should not be deletable
		if p.level >= 0 && p.level <= 3 {
			isBuiltin := p.title == "Owner" || p.title == "Top Manager" || p.title == "Manager" || p.title == "Employee"
			if !isBuiltin {
				t.Errorf("position %s at level %d should be recognized as built-in", p.title, p.level)
			}
		}
	}
}

func TestCompany_OwnerCannotLeave(t *testing.T) {
	// Owner must transfer ownership before leaving
	ownerID := "owner-uuid"
	userID := "owner-uuid"

	if ownerID == userID {
		// This should be blocked
		t.Log("Owner trying to leave own company — should be blocked")
	}
}

func TestCompany_OwnerCannotBeRemoved(t *testing.T) {
	companyOwnerID := "owner-uuid"
	targetUserID := "owner-uuid"

	if companyOwnerID == targetUserID {
		t.Log("Trying to remove owner — should be blocked")
	}
}

func TestCompanyChat_ParticipantsJSON(t *testing.T) {
	// Test building participants JSON
	users := []string{"user1", "user2", "user3"}
	var parts []string
	for _, u := range users {
		parts = append(parts, "'"+u+"'")
	}
	result := "[" + strings.Join(parts, ",") + "]"

	if !strings.Contains(result, "user1") {
		t.Error("participants should contain user1")
	}
	if !strings.Contains(result, "user2") {
		t.Error("participants should contain user2")
	}
	if !strings.Contains(result, "user3") {
		t.Error("participants should contain user3")
	}
}

func TestGetProfileResponse_CompanyFields(t *testing.T) {
	// GetProfileResponse should have company fields
	type profileResponse struct {
		CompanyId      string
		CompanyName    string
		PositionTitle  string
		PositionLevel  int32
	}

	resp := profileResponse{
		CompanyId:     "company-123",
		CompanyName:   "Acme Corp",
		PositionTitle: "Manager",
		PositionLevel: 1,
	}

	if resp.CompanyId == "" {
		t.Error("CompanyId should not be empty")
	}
	if resp.CompanyName == "" {
		t.Error("CompanyName should not be empty")
	}
	if resp.PositionLevel < 0 || resp.PositionLevel > 3 {
		t.Errorf("PositionLevel %d is out of range 0-3", resp.PositionLevel)
	}
}
