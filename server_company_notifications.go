package main

import (
	"LavenderMessenger/gen"
	"context"
	"fmt"
	"time"

	"firebase.google.com/go/v4/messaging"
)

func (c *companyServer) SendCompanyNotification(ctx context.Context, req *gen.SendCompanyNotificationRequest) (*gen.SendCompanyNotificationResponse, error) {
	userID := GetUserID(ctx)
	if userID == "" {
		return &gen.SendCompanyNotificationResponse{Success: false}, nil
	}

	// Get company name
	var companyName string
	_ = c.db.QueryRow(`SELECT name FROM companies WHERE id=$1::uuid`, req.CompanyId).Scan(&companyName)

	// Get all member user IDs and FCM tokens (excluding actor)
	rows, err := c.db.Query(`
		SELECT u.id, COALESCE(ut.fcm_token, ''), COALESCE(ut.push_enabled, FALSE)
		FROM company_members cm
		JOIN users u ON u.id = cm.user_id
		LEFT JOIN user_tokens ut ON ut.user_id = u.id
		WHERE cm.company_id=$1::uuid AND u.id != $2::uuid`, req.CompanyId, userID)
	if err != nil {
		logger.Errorf("Company: SendCompanyNotification query error: %v", err)
		return &gen.SendCompanyNotificationResponse{Success: false}, nil
	}
	defer rows.Close()

	eventText := formatCompanyEvent(req.EventType, req.ActorUsername, req.TargetUsername, req.PositionName, companyName)

	var sentCount int
	for rows.Next() {
		var memberID, fcmToken string
		var pushEnabled bool
		if err := rows.Scan(&memberID, &fcmToken, &pushEnabled); err != nil {
			continue
		}
		if !pushEnabled || fcmToken == "" {
			continue
		}
		c.sendCompanyEventPush(fcmToken, req.CompanyId, companyName, req.EventType.String(), eventText)
		sentCount++
	}

	logger.Infof("Company: Notification sent to %d members of company %s (event=%s)", sentCount, req.CompanyId, req.EventType)

	return &gen.SendCompanyNotificationResponse{Success: true}, nil
}

func formatCompanyEvent(eventType gen.CompanyEventType, actor, target, position, company string) string {
	switch eventType {
	case gen.CompanyEventType_MEMBER_JOINED:
		return fmt.Sprintf("%s joined %s", actor, company)
	case gen.CompanyEventType_MEMBER_LEFT:
		return fmt.Sprintf("%s left %s", actor, company)
	case gen.CompanyEventType_POSITION_CHANGED:
		if target != "" && position != "" {
			return fmt.Sprintf("%s changed %s's position to %s", actor, target, position)
		}
		return fmt.Sprintf("Position changed in %s", company)
	case gen.CompanyEventType_COMPANY_CHAT_CREATED:
		return fmt.Sprintf("%s created a chat in %s", actor, company)
	default:
		return fmt.Sprintf("Event in %s", company)
	}
}

func (c *companyServer) sendCompanyEventPush(token, companyID, companyName, eventType, body string) {
	if firebaseApp == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := firebaseApp.Messaging(ctx)
	if err != nil {
		return
	}

	title := "Company"

	msg := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: buildSafeDataMap(map[string]string{
			"title":      title,
			"body":       body,
			"type":       "company_event",
			"company_id": companyID,
			"event_type": eventType,
		}),
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: "lavender_messages_v2",
				Priority:  messaging.PriorityHigh,
			},
		},
	}

	_, err = client.Send(ctx, msg)
	if err != nil {
		logger.Errorf("Company: FCM push error: %v", err)
	}
}
