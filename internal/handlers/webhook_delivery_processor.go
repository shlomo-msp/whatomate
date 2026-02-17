package handlers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/shridarpatil/whatomate/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	webhookStatusPending    = "pending"
	webhookStatusInProgress = "in_progress"
	webhookStatusDelivered  = "delivered"
	webhookStatusFailed     = "failed"
)

var webhookRetrySchedule = []time.Duration{
	time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	time.Hour,
	6 * time.Hour,
	24 * time.Hour,
}

// WebhookDeliveryProcessor delivers outbound webhooks from the outbox.
type WebhookDeliveryProcessor struct {
	app      *App
	interval time.Duration
	stopCh   chan struct{}
}

// NewWebhookDeliveryProcessor creates a new webhook delivery processor.
func NewWebhookDeliveryProcessor(app *App, interval time.Duration) *WebhookDeliveryProcessor {
	return &WebhookDeliveryProcessor{
		app:      app,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the webhook delivery loop.
func (p *WebhookDeliveryProcessor) Start(ctx context.Context) {
	p.app.Log.Info("Webhook delivery processor started", "interval", p.interval)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.app.Log.Info("Webhook delivery processor stopped by context")
			return
		case <-p.stopCh:
			p.app.Log.Info("Webhook delivery processor stopped")
			return
		case <-ticker.C:
			p.processPendingDeliveries()
		}
	}
}

// Stop stops the webhook delivery processor.
func (p *WebhookDeliveryProcessor) Stop() {
	close(p.stopCh)
}

func (p *WebhookDeliveryProcessor) processPendingDeliveries() {
	now := time.Now().UTC()
	staleCutoff := now.Add(-15 * time.Minute)
	batchSize := 50

	for {
		var deliveries []models.WebhookDelivery
		err := p.app.DB.Transaction(func(tx *gorm.DB) error {
			query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
				Where("status = ? AND next_attempt_at <= ?", webhookStatusPending, now).
				Or("status = ? AND processing_started_at <= ?", webhookStatusInProgress, staleCutoff).
				Order("next_attempt_at ASC").
				Limit(batchSize)

			if err := query.Find(&deliveries).Error; err != nil {
				return err
			}
			if len(deliveries) == 0 {
				return nil
			}

			ids := make([]interface{}, 0, len(deliveries))
			for _, d := range deliveries {
				ids = append(ids, d.ID)
			}
			return tx.Model(&models.WebhookDelivery{}).
				Where("id IN ?", ids).
				Updates(map[string]interface{}{
					"status":                webhookStatusInProgress,
					"processing_started_at": now,
				}).Error
		})

		if err != nil {
			p.app.Log.Error("Failed to load webhook deliveries", "error", err)
			return
		}

		if len(deliveries) == 0 {
			return
		}

		for _, delivery := range deliveries {
			p.app.processWebhookDelivery(delivery)
		}
	}
}

func (a *App) processWebhookDelivery(delivery models.WebhookDelivery) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Reload the delivery to pick up any updated URL/headers/secret/payload.
	var fresh models.WebhookDelivery
	if err := a.DB.Where("id = ?", delivery.ID).First(&fresh).Error; err == nil {
		delivery = fresh
	}

	jsonData, err := json.Marshal(delivery.Payload)
	if err != nil {
		a.failWebhookDelivery(delivery, 0, "failed to marshal payload: "+err.Error())
		return
	}

	err = a.sendWebhookRequest(ctx, delivery.URL, delivery.Headers, delivery.Secret, jsonData)
	if err == nil {
		now := time.Now().UTC()
		updates := map[string]interface{}{
			"status":                webhookStatusDelivered,
			"delivered_at":          &now,
			"processing_started_at": nil,
			"last_error":            "",
			"last_status_code":      0,
		}
		if err := a.DB.Model(&models.WebhookDelivery{}).Where("id = ?", delivery.ID).Updates(updates).Error; err != nil {
			a.Log.Error("Failed to update delivered webhook", "error", err, "delivery_id", delivery.ID)
		}
		return
	}

	statusCode := 0
	if whErr, ok := err.(*WebhookError); ok {
		statusCode = whErr.StatusCode
	}
	a.failWebhookDelivery(delivery, statusCode, err.Error())
}

func (a *App) failWebhookDelivery(delivery models.WebhookDelivery, statusCode int, errMsg string) {
	attempts := delivery.Attempts + 1
	maxAttempts := delivery.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = len(webhookRetrySchedule)
	}

	status := webhookStatusPending
	nextAttempt := time.Now().UTC().Add(nextWebhookAttemptDelay(attempts))
	if attempts >= maxAttempts {
		status = webhookStatusFailed
	}

	updates := map[string]interface{}{
		"status":                status,
		"attempts":              attempts,
		"last_error":            errMsg,
		"last_status_code":      statusCode,
		"processing_started_at": nil,
	}
	if status == webhookStatusPending {
		updates["next_attempt_at"] = nextAttempt
	}

	if err := a.DB.Model(&models.WebhookDelivery{}).Where("id = ?", delivery.ID).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to update webhook delivery failure", "error", err, "delivery_id", delivery.ID)
	}
}

func nextWebhookAttemptDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return webhookRetrySchedule[0]
	}
	idx := attempt - 1
	if idx >= len(webhookRetrySchedule) {
		return webhookRetrySchedule[len(webhookRetrySchedule)-1]
	}
	return webhookRetrySchedule[idx]
}
