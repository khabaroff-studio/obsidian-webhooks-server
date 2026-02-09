package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MailerLiteService handles subscriber management via MailerLite API
type MailerLiteService struct {
	apiKey       string
	groupSignups string // Group ID for new signups
	groupActive  string // Group ID for active users
	httpClient   *http.Client
	baseURL      string
}

// NewMailerLiteService creates a new MailerLite service
func NewMailerLiteService(apiKey, groupSignups, groupActive string) *MailerLiteService {
	return &MailerLiteService{
		apiKey:       apiKey,
		groupSignups: groupSignups,
		groupActive:  groupActive,
		httpClient: &http.Client{
			Timeout: time.Second * 30,
		},
		baseURL: "https://connect.mailerlite.com/api",
	}
}

// SubscriberStatus represents the status of a subscriber
type SubscriberStatus string

const (
	SubscriberStatusActive     SubscriberStatus = "active"
	SubscriberStatusUnsubscribed SubscriberStatus = "unsubscribed"
	SubscriberStatusUnconfirmed  SubscriberStatus = "unconfirmed"
	SubscriberStatusBounced      SubscriberStatus = "bounced"
	SubscriberStatusJunk         SubscriberStatus = "junk"
)

// AddSubscriberRequest represents a request to add a subscriber
type AddSubscriberRequest struct {
	Email  string            `json:"email"`
	Fields map[string]string `json:"fields,omitempty"`
	Groups []string          `json:"groups,omitempty"`
	Status SubscriberStatus  `json:"status,omitempty"`
}

// SubscriberResponse represents a MailerLite subscriber response
type SubscriberResponse struct {
	ID     string           `json:"id"`
	Email  string           `json:"email"`
	Status SubscriberStatus `json:"status"`
	Groups []string         `json:"groups"`
}

// AddSubscriber adds a new subscriber to MailerLite (signup group)
// language: "en" or "ru" for segmentation
func (s *MailerLiteService) AddSubscriber(ctx context.Context, email, name, language string) error {
	fields := make(map[string]string)
	if name != "" {
		fields["name"] = name
	}
	// Add language field for segmentation in MailerLite
	if language != "" {
		fields["language"] = language
	} else {
		fields["language"] = "en" // Default to English
	}

	groups := []string{}
	if s.groupSignups != "" {
		groups = append(groups, s.groupSignups)
	}

	req := AddSubscriberRequest{
		Email:  email,
		Fields: fields,
		Groups: groups,
		Status: SubscriberStatusActive,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal subscriber request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/subscribers", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create subscriber request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send subscriber request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("MailerLite API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// UpdateSubscriberGroup moves a subscriber to a different group (e.g., signup → active)
func (s *MailerLiteService) UpdateSubscriberGroup(ctx context.Context, email, newGroupID string) error {
	// First, get subscriber ID by email
	subscriberID, err := s.getSubscriberIDByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("failed to get subscriber ID: %w", err)
	}

	// Add subscriber to new group
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		fmt.Sprintf("%s/subscribers/%s/groups/%s", s.baseURL, subscriberID, newGroupID),
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create group update request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send group update request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("MailerLite API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// MoveToActiveGroup moves a subscriber from signup group to active group
func (s *MailerLiteService) MoveToActiveGroup(ctx context.Context, email string) error {
	if s.groupActive == "" {
		return nil // Active group not configured, skip
	}

	return s.UpdateSubscriberGroup(ctx, email, s.groupActive)
}

// UpdateSubscriberStatus updates a subscriber's status (active, unsubscribed, etc.)
func (s *MailerLiteService) UpdateSubscriberStatus(ctx context.Context, email string, status SubscriberStatus) error {
	// Get subscriber ID by email
	subscriberID, err := s.getSubscriberIDByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("failed to get subscriber ID: %w", err)
	}

	// Update status
	updateReq := map[string]interface{}{
		"status": status,
	}

	body, err := json.Marshal(updateReq)
	if err != nil {
		return fmt.Errorf("failed to marshal status update: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		"PUT",
		fmt.Sprintf("%s/subscribers/%s", s.baseURL, subscriberID),
		bytes.NewBuffer(body),
	)
	if err != nil {
		return fmt.Errorf("failed to create status update request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send status update request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("MailerLite API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// getSubscriberIDByEmail fetches subscriber ID by email address
func (s *MailerLiteService) getSubscriberIDByEmail(ctx context.Context, email string) (string, error) {
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("%s/subscribers?filter[email]=%s", s.baseURL, email),
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create subscriber lookup request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send subscriber lookup request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("MailerLite API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []SubscriberResponse `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode subscriber response: %w", err)
	}

	if len(result.Data) == 0 {
		return "", fmt.Errorf("subscriber not found: %s", email)
	}

	return result.Data[0].ID, nil
}
