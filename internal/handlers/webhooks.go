package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// validateWebhookURL performs structural validation of a webhook URL.
// It blocks known-internal hostnames and IP literals pointing to private ranges.
// Runtime SSRF protection (DNS rebinding) is handled by SSRFSafeDialer.
func validateWebhookURL(rawURL string, allowInternal bool) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("URL scheme must be http or https")
	}

	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	if !allowInternal {
		// Block obvious internal hostnames
		lower := strings.ToLower(hostname)
		if lower == "localhost" || lower == "0.0.0.0" || strings.HasSuffix(lower, ".local") ||
			strings.HasSuffix(lower, ".internal") {
			return fmt.Errorf("URL must not point to internal addresses")
		}

		// Block private/loopback IP literals (e.g. http://127.0.0.1, http://[::1])
		if ip := net.ParseIP(hostname); ip != nil {
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
				ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
				return fmt.Errorf("URL must not point to internal addresses")
			}
		}
	}

	return nil
}

// SSRFSafeDialer returns a DialContext function that blocks connections to
// private/loopback IPs after DNS resolution. Use this in http.Transport
// for webhook and custom action HTTP calls.
func SSRFSafeDialer(allowInternal bool) func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if allowInternal {
		return dialer.DialContext
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		ips, err := net.DefaultResolver.LookupHost(ctx, host)
		if err != nil {
			return nil, err
		}

		for _, ipStr := range ips {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				continue
			}
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
				ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
				return nil, fmt.Errorf("connection to private address %s is not allowed", ipStr)
			}
		}

		// Connect to first resolved IP
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0], port))
	}
}

// WebhookRequest represents the request body for creating/updating a webhook
type WebhookRequest struct {
	Name     string            `json:"name"`
	URL      string            `json:"url"`
	Events   []string          `json:"events"`
	Headers  map[string]string `json:"headers"`
	Secret   string            `json:"secret"`
	IsActive bool              `json:"is_active"`
}

// WebhookResponse represents the API response for a webhook
type WebhookResponse struct {
	ID        uuid.UUID         `json:"id"`
	Name      string            `json:"name"`
	URL       string            `json:"url"`
	Events    []string          `json:"events"`
	Headers   map[string]string `json:"headers"`
	IsActive  bool              `json:"is_active"`
	HasSecret bool              `json:"has_secret"`
	FailedCount int64           `json:"failed_count"`
	RetryingCount int64         `json:"retrying_count"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

// AvailableWebhookEvents returns the list of available webhook event types
var AvailableWebhookEvents = []map[string]string{
	{"value": string(models.WebhookEventMessageIncoming), "label": "Message Incoming", "description": "When a new message is received from a contact"},
	{"value": string(models.WebhookEventMessageSent), "label": "Message Sent", "description": "When an agent sends a message"},
	{"value": string(models.WebhookEventContactCreated), "label": "Contact Created", "description": "When a new contact is created"},
	{"value": string(models.WebhookEventTransferCreated), "label": "Transfer Created", "description": "When a transfer to human agent is requested"},
	{"value": string(models.WebhookEventTransferAssigned), "label": "Transfer Assigned", "description": "When a transfer is assigned to an agent"},
	{"value": string(models.WebhookEventTransferResumed), "label": "Transfer Resumed", "description": "When chatbot is resumed (transfer closed)"},
}

// ListWebhooks returns all webhooks for the organization
func (a *App) ListWebhooks(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	pg := parsePagination(r)
	search := string(r.RequestCtx.QueryArgs().Peek("search"))

	query := a.DB.Where("organization_id = ?", orgID)

	// Apply search filter - search by name or URL (case-insensitive)
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("name ILIKE ? OR url ILIKE ?", searchPattern, searchPattern)
	}

	var total int64
	query.Model(&models.Webhook{}).Count(&total)

	var webhooks []models.Webhook
	if err := pg.Apply(query.Model(&models.Webhook{}).Order("created_at DESC")).
		Find(&webhooks).Error; err != nil {
		a.Log.Error("Failed to list webhooks", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list webhooks", nil, "")
	}

	failedCounts := map[uuid.UUID]int64{}
	retryingCounts := map[uuid.UUID]int64{}
	var rows []struct {
		WebhookID uuid.UUID `gorm:"column:webhook_id"`
		Count     int64     `gorm:"column:count"`
	}
	if err := a.DB.Model(&models.WebhookDelivery{}).
		Select("webhook_id, COUNT(*) as count").
		Where("organization_id = ? AND status = ?", orgID, "failed").
		Group("webhook_id").
		Scan(&rows).Error; err == nil {
		for _, row := range rows {
			failedCounts[row.WebhookID] = row.Count
		}
	}
	if err := a.DB.Model(&models.WebhookDelivery{}).
		Select("webhook_id, COUNT(*) as count").
		Where("organization_id = ? AND status IN ? AND last_error <> ''", orgID, []string{"pending", "in_progress"}).
		Group("webhook_id").
		Scan(&rows).Error; err == nil {
		for _, row := range rows {
			retryingCounts[row.WebhookID] = row.Count
		}
	}

	result := make([]WebhookResponse, len(webhooks))
	for i, wh := range webhooks {
		result[i] = webhookToResponse(wh, failedCounts[wh.ID], retryingCounts[wh.ID])
	}

	return r.SendEnvelope(map[string]any{
		"webhooks":         result,
		"available_events": AvailableWebhookEvents,
		"total":            total,
		"page":             pg.Page,
		"limit":            pg.Limit,
	})
}

// GetWebhook returns a single webhook by ID
func (a *App) GetWebhook(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	webhookID, err := parsePathUUID(r, "id", "webhook")
	if err != nil {
		return nil
	}

	webhook, err := findByIDAndOrg[models.Webhook](a.DB, r, webhookID, orgID, "Webhook")
	if err != nil {
		return nil
	}

	var failedCount int64
	_ = a.DB.Model(&models.WebhookDelivery{}).
		Where("organization_id = ? AND webhook_id = ? AND status = ?", orgID, webhookID, "failed").
		Count(&failedCount).Error
	var retryingCount int64
	_ = a.DB.Model(&models.WebhookDelivery{}).
		Where("organization_id = ? AND webhook_id = ? AND status IN ? AND last_error <> ''", orgID, webhookID, []string{"pending", "in_progress"}).
		Count(&retryingCount).Error

	return r.SendEnvelope(webhookToResponse(*webhook, failedCount, retryingCount))
}

// CreateWebhook creates a new webhook
func (a *App) CreateWebhook(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req WebhookRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.Name == "" || req.URL == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "name and url are required", nil, "")
	}

	if err := validateWebhookURL(req.URL, a.Config.App.AllowInternalWebhookURLs); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	if len(req.Events) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "at least one event must be selected", nil, "")
	}

	// Convert headers to JSONB
	headers := models.JSONB{}
	for k, v := range req.Headers {
		headers[k] = v
	}

	// Auto-generate secret if not provided
	secret := req.Secret
	if secret == "" {
		secret = generateVerifyToken() // Reuse the 32-byte hex generator
	}

	webhook := models.Webhook{
		OrganizationID: orgID,
		Name:           req.Name,
		URL:            req.URL,
		Events:         req.Events,
		Headers:        headers,
		Secret:         secret,
		IsActive:       true,
	}

	if err := a.DB.Create(&webhook).Error; err != nil {
		a.Log.Error("Failed to create webhook", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create webhook", nil, "")
	}

	// Invalidate cache
	a.InvalidateWebhooksCache(orgID)

	return r.SendEnvelope(webhookToResponse(webhook, 0, 0))
}

// UpdateWebhook updates an existing webhook
func (a *App) UpdateWebhook(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	webhookID, err := parsePathUUID(r, "id", "webhook")
	if err != nil {
		return nil
	}

	webhook, err := findByIDAndOrg[models.Webhook](a.DB, r, webhookID, orgID, "Webhook")
	if err != nil {
		return nil
	}

	var req WebhookRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.Name != "" {
		webhook.Name = req.Name
	}
	if req.URL != "" {
		if err := validateWebhookURL(req.URL, a.Config.App.AllowInternalWebhookURLs); err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
		}
		webhook.URL = req.URL
	}
	if len(req.Events) > 0 {
		webhook.Events = req.Events
	}

	// Update headers if provided
	if req.Headers != nil {
		headers := models.JSONB{}
		for k, v := range req.Headers {
			headers[k] = v
		}
		webhook.Headers = headers
	}

	// Update secret if provided (empty string clears it)
	if req.Secret != "" {
		webhook.Secret = req.Secret
	}

	webhook.IsActive = req.IsActive

	if err := a.DB.Save(webhook).Error; err != nil {
		a.Log.Error("Failed to update webhook", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update webhook", nil, "")
	}

	// Invalidate cache
	a.InvalidateWebhooksCache(orgID)

	return r.SendEnvelope(webhookToResponse(*webhook, 0, 0))
}

// DeleteWebhook deletes a webhook
func (a *App) DeleteWebhook(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	webhookID, err := parsePathUUID(r, "id", "webhook")
	if err != nil {
		return nil
	}

	result := a.DB.Where("id = ? AND organization_id = ?", webhookID, orgID).Delete(&models.Webhook{})
	if result.Error != nil {
		a.Log.Error("Failed to delete webhook", "error", result.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete webhook", nil, "")
	}
	if result.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Webhook not found", nil, "")
	}

	// Invalidate cache
	a.InvalidateWebhooksCache(orgID)

	return r.SendEnvelope(map[string]string{"message": "Webhook deleted successfully"})
}

// TestWebhook sends a test event to a webhook
func (a *App) TestWebhook(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	webhookID, err := parsePathUUID(r, "id", "webhook")
	if err != nil {
		return nil
	}

	webhook, err := findByIDAndOrg[models.Webhook](a.DB, r, webhookID, orgID, "Webhook")
	if err != nil {
		return nil
	}

	// Send a test event synchronously
	testData := map[string]interface{}{
		"test":      true,
		"message":   "This is a test webhook from Whatomate",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	payload := OutboundWebhookPayload{
		DeliveryID: uuid.New().String(),
		Event:      "test",
		Timestamp:  time.Now().UTC(),
		Data:       testData,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create test payload", nil, "")
	}

	// Use timeout context for test webhook request
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := a.sendWebhookRequest(ctx, webhook.URL, webhook.Headers, webhook.Secret, jsonData); err != nil {
		a.Log.Error("Webhook test failed", "error", err, "webhook_id", webhook.ID)
		return r.SendErrorEnvelope(fasthttp.StatusBadGateway, "Webhook test failed", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Test webhook sent successfully"})
}

// RetryFailedWebhookDeliveries resets failed deliveries for a webhook
func (a *App) RetryFailedWebhookDeliveries(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	webhookID, err := parsePathUUID(r, "id", "webhook")
	if err != nil {
		return nil
	}

	if _, err := findByIDAndOrg[models.Webhook](a.DB, r, webhookID, orgID, "Webhook"); err != nil {
		return nil
	}

	now := time.Now().UTC()
	result := a.DB.Model(&models.WebhookDelivery{}).
		Where("organization_id = ? AND webhook_id = ? AND (status = ? OR (status IN ? AND last_error <> ''))",
			orgID, webhookID, "failed", []string{"pending", "in_progress"}).
		Updates(map[string]interface{}{
			"status":                "pending",
			"next_attempt_at":       now,
			"processing_started_at": nil,
			"last_error":            "",
			"last_status_code":      0,
		})

	if result.Error != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to retry webhook deliveries", nil, "")
	}

	return r.SendEnvelope(map[string]any{
		"message": "Retry scheduled",
		"count":   result.RowsAffected,
	})
}

func webhookToResponse(wh models.Webhook, failedCount int64, retryingCount int64) WebhookResponse {
	// Convert events
	events := make([]string, len(wh.Events))
	copy(events, wh.Events)

	// Convert headers
	headers := make(map[string]string)
	for k, v := range wh.Headers {
		if strVal, ok := v.(string); ok {
			headers[k] = strVal
		}
	}

	return WebhookResponse{
		ID:        wh.ID,
		Name:      wh.Name,
		URL:       wh.URL,
		Events:    events,
		Headers:   headers,
		IsActive:  wh.IsActive,
		HasSecret: wh.Secret != "",
		FailedCount: failedCount,
		RetryingCount: retryingCount,
		CreatedAt: wh.CreatedAt.Format(time.RFC3339),
		UpdatedAt: wh.UpdatedAt.Format(time.RFC3339),
	}
}
