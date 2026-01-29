package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
)


// OutboundWebhookPayload represents the structure sent to external webhook endpoints
type OutboundWebhookPayload struct {
	DeliveryID string      `json:"delivery_id"`
	Event      string      `json:"event"`
	Timestamp  time.Time   `json:"timestamp"`
	Data       interface{} `json:"data"`
}

// MessageEventData represents data for message events
type MessageEventData struct {
	MessageID       string             `json:"message_id"`
	ContactID       string             `json:"contact_id"`
	ContactPhone    string             `json:"contact_phone"`
	ContactName     string             `json:"contact_name"`
	MessageType     models.MessageType `json:"message_type"`
	Content         string             `json:"content"`
	MediaFilename   string             `json:"media_filename,omitempty"`
	WhatsAppAccount string             `json:"whatsapp_account"`
	Direction       models.Direction   `json:"direction,omitempty"`
	SentByUserID    string             `json:"sent_by_user_id,omitempty"`
}

// ContactEventData represents data for contact events
type ContactEventData struct {
	ContactID       string `json:"contact_id"`
	ContactPhone    string `json:"contact_phone"`
	ContactName     string `json:"contact_name"`
	WhatsAppAccount string `json:"whatsapp_account"`
}

// TransferEventData represents data for transfer events
type TransferEventData struct {
	TransferID      string                `json:"transfer_id"`
	ContactID       string                `json:"contact_id"`
	ContactPhone    string                `json:"contact_phone"`
	ContactName     string                `json:"contact_name"`
	Source          models.TransferSource `json:"source"`
	Reason          string                `json:"reason,omitempty"`
	AgentID         *string               `json:"agent_id,omitempty"`
	AgentName       *string               `json:"agent_name,omitempty"`
	WhatsAppAccount string                `json:"whatsapp_account"`
}

// DispatchWebhook sends an event to all matching webhooks for the organization
func (a *App) DispatchWebhook(orgID uuid.UUID, eventType models.WebhookEvent, data interface{}) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		// Use detached context with timeout for webhook delivery
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		a.enqueueWebhookDeliveries(ctx, orgID, string(eventType), data)
	}()
}

func (a *App) enqueueWebhookDeliveries(ctx context.Context, orgID uuid.UUID, eventType string, data interface{}) {
	// Find all active webhooks for this org that subscribe to this event (use cache)
	webhooks, err := a.getWebhooksCached(orgID)
	if err != nil {
		a.Log.Error("failed to fetch webhooks", "error", err)
		return
	}

	for _, webhook := range webhooks {
		// Check if webhook subscribes to this event
		if !containsEvent(webhook.Events, eventType) {
			continue
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			a.Log.Warn("webhook enqueue cancelled", "reason", ctx.Err())
			break
		}

		deliveryID := uuid.New()
		payload := OutboundWebhookPayload{
			DeliveryID: deliveryID.String(),
			Event:      eventType,
			Timestamp:  time.Now().UTC(),
			Data:       data,
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			a.Log.Error("failed to marshal webhook payload", "error", err, "webhook_id", webhook.ID)
			continue
		}

		var payloadMap models.JSONB
		if err := json.Unmarshal(jsonData, &payloadMap); err != nil {
			a.Log.Error("failed to decode webhook payload", "error", err, "webhook_id", webhook.ID)
			continue
		}

		delivery := models.WebhookDelivery{
			BaseModel:     models.BaseModel{ID: deliveryID},
			OrganizationID: orgID,
			WebhookID:     webhook.ID,
			Event:         eventType,
			URL:           webhook.URL,
			Headers:       webhook.Headers,
			Secret:        webhook.Secret,
			Payload:       payloadMap,
			Status:        "pending",
			Attempts:      0,
			MaxAttempts:   6,
			NextAttemptAt: time.Now().UTC(),
		}

		if err := a.DB.Create(&delivery).Error; err != nil {
			a.Log.Error("failed to enqueue webhook delivery", "error", err, "webhook_id", webhook.ID)
			continue
		}

		// Mark in progress and attempt immediate send
		startedAt := time.Now().UTC()
		if err := a.DB.Model(&models.WebhookDelivery{}).
			Where("id = ?", delivery.ID).
			Updates(map[string]interface{}{
				"status":                webhookStatusInProgress,
				"processing_started_at": startedAt,
			}).Error; err != nil {
			a.Log.Error("failed to mark webhook delivery in progress", "error", err, "delivery_id", delivery.ID)
			continue
		}
		delivery.Status = webhookStatusInProgress
		delivery.ProcessingStartedAt = &startedAt

		a.wg.Add(1)
		go func(d models.WebhookDelivery) {
			defer a.wg.Done()
			a.processWebhookDelivery(d)
		}(delivery)
	}
}

func containsEvent(events models.StringArray, event string) bool {
	for _, e := range events {
		if e == event {
			return true
		}
	}
	return false
}

func (a *App) sendWebhookRequest(ctx context.Context, url string, headers models.JSONB, secret string, jsonData []byte) error {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Whatomate-Webhook/1.0")

	// Add custom headers from webhook config
	if headers != nil {
		for key, value := range headers {
			if strValue, ok := value.(string); ok {
				req.Header.Set(key, strValue)
			}
		}
	}

	// Add HMAC signature if secret is configured
	if secret != "" {
		signature := computeHMACSignature(jsonData, secret)
		req.Header.Set("X-Webhook-Signature", signature)
	}

	// Send request
	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for successful status code (2xx)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &WebhookError{StatusCode: resp.StatusCode}
	}

	return nil
}

func computeHMACSignature(data []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(data)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// WebhookError represents a webhook delivery error
type WebhookError struct {
	StatusCode int
}

func (e *WebhookError) Error() string {
	return "webhook returned non-2xx status: " + http.StatusText(e.StatusCode)
}
