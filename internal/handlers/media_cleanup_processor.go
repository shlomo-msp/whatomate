package handlers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
)

// MediaCleanupProcessor deletes old local media files based on org settings.
type MediaCleanupProcessor struct {
	app      *App
	interval time.Duration
	stopCh   chan struct{}
}

// NewMediaCleanupProcessor creates a new media cleanup processor.
func NewMediaCleanupProcessor(app *App, interval time.Duration) *MediaCleanupProcessor {
	return &MediaCleanupProcessor{
		app:      app,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the media cleanup loop.
func (p *MediaCleanupProcessor) Start(ctx context.Context) {
	p.app.Log.Info("Media cleanup processor started", "interval", p.interval)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.app.Log.Info("Media cleanup processor stopped by context")
			return
		case <-p.stopCh:
			p.app.Log.Info("Media cleanup processor stopped")
			return
		case <-ticker.C:
			p.processStaleMedia()
		}
	}
}

// Stop stops the media cleanup processor.
func (p *MediaCleanupProcessor) Stop() {
	close(p.stopCh)
}

func (p *MediaCleanupProcessor) processStaleMedia() {
	now := time.Now()

	var orgs []models.Organization
	if err := p.app.DB.Select("id, settings").Find(&orgs).Error; err != nil {
		p.app.Log.Error("Failed to load organizations for media cleanup", "error", err)
		return
	}

	for _, org := range orgs {
		enabled, days := getOrgMediaCleanupSettings(org)
		if !enabled || days <= 0 {
			continue
		}

		cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
		deletedCount, checkedCount := p.cleanupOrganizationMedia(org.ID, cutoff)

		if deletedCount > 0 {
			p.app.Log.Info("Media cleanup completed",
				"org_id", org.ID,
				"deleted", deletedCount,
				"checked", checkedCount,
				"cutoff", cutoff,
			)
		}
	}
}

func getOrgMediaCleanupSettings(org models.Organization) (bool, int) {
	enabled := false
	days := 30

	if org.Settings != nil {
		if v, ok := org.Settings["auto_delete_media_enabled"].(bool); ok {
			enabled = v
		}
		if v, ok := org.Settings["auto_delete_media_days"].(float64); ok && v > 0 {
			days = int(v)
		}
	}

	return enabled, days
}

func (p *MediaCleanupProcessor) cleanupOrganizationMedia(orgID uuid.UUID, cutoff time.Time) (int, int) {
	paths := make(map[string]struct{})

	var messagePaths []string
	if err := p.app.DB.Model(&models.Message{}).
		Where("organization_id = ? AND media_url <> ''", orgID).
		Pluck("media_url", &messagePaths).Error; err != nil {
		p.app.Log.Error("Failed to load message media paths", "error", err, "org_id", orgID)
		return 0, 0
	}
	for _, path := range messagePaths {
		if path != "" {
			paths[path] = struct{}{}
		}
	}

	var campaignPaths []string
	if err := p.app.DB.Model(&models.BulkMessageCampaign{}).
		Where("organization_id = ? AND header_media_local_path <> ''", orgID).
		Pluck("header_media_local_path", &campaignPaths).Error; err != nil {
		p.app.Log.Error("Failed to load campaign media paths", "error", err, "org_id", orgID)
		return 0, 0
	}
	for _, path := range campaignPaths {
		if path != "" {
			paths[path] = struct{}{}
		}
	}

	basePath := p.app.getMediaStoragePath()
	baseAbs, err := filepath.Abs(basePath)
	if err != nil {
		p.app.Log.Error("Failed to resolve media base path", "error", err, "base_path", basePath)
		return 0, 0
	}

	checked := 0
	deleted := 0

	for relPath := range paths {
		checked++
		if strings.Contains(relPath, "..") {
			p.app.Log.Warn("Skipping suspicious media path", "org_id", orgID, "path", relPath)
			continue
		}

		fullPath := filepath.Join(baseAbs, relPath)
		fullAbs, err := filepath.Abs(fullPath)
		if err != nil {
			p.app.Log.Warn("Failed to resolve media path", "org_id", orgID, "path", relPath, "error", err)
			continue
		}

		if !strings.HasPrefix(fullAbs, baseAbs+string(os.PathSeparator)) && fullAbs != baseAbs {
			p.app.Log.Warn("Skipping media path outside base", "org_id", orgID, "path", relPath)
			continue
		}

		info, err := os.Stat(fullAbs)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			p.app.Log.Warn("Failed to stat media file", "org_id", orgID, "path", relPath, "error", err)
			continue
		}
		if info.IsDir() {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}

		if err := os.Remove(fullAbs); err != nil {
			p.app.Log.Warn("Failed to delete media file", "org_id", orgID, "path", relPath, "error", err)
			continue
		}
		deleted++
	}

	return deleted, checked
}
