package handlers

import (
	"encoding/json"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// OrganizationSettings represents the settings structure
type OrganizationSettings struct {
	MaskPhoneNumbers bool   `json:"mask_phone_numbers"`
	Timezone         string `json:"timezone"`
	DateFormat       string `json:"date_format"`
}

// GetOrganizationSettings returns the organization settings
func (a *App) GetOrganizationSettings(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Organization not found", nil, "")
	}

	// Parse settings from JSONB
	settings := OrganizationSettings{
		MaskPhoneNumbers: false,
		Timezone:         "UTC",
		DateFormat:       "YYYY-MM-DD",
	}

	if org.Settings != nil {
		if v, ok := org.Settings["mask_phone_numbers"].(bool); ok {
			settings.MaskPhoneNumbers = v
		}
		if v, ok := org.Settings["timezone"].(string); ok && v != "" {
			settings.Timezone = v
		}
		if v, ok := org.Settings["date_format"].(string); ok && v != "" {
			settings.DateFormat = v
		}
	}

	return r.SendEnvelope(map[string]interface{}{
		"settings": settings,
		"name":     org.Name,
	})
}

// UpdateOrganizationSettings updates the organization settings
func (a *App) UpdateOrganizationSettings(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req struct {
		MaskPhoneNumbers *bool   `json:"mask_phone_numbers"`
		Timezone         *string `json:"timezone"`
		DateFormat       *string `json:"date_format"`
		Name             *string `json:"name"`
	}

	if err := json.Unmarshal(r.RequestCtx.PostBody(), &req); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Organization not found", nil, "")
	}

	// Update settings
	if org.Settings == nil {
		org.Settings = models.JSONB{}
	}

	if req.MaskPhoneNumbers != nil {
		org.Settings["mask_phone_numbers"] = *req.MaskPhoneNumbers
	}
	if req.Timezone != nil {
		org.Settings["timezone"] = *req.Timezone
	}
	if req.DateFormat != nil {
		org.Settings["date_format"] = *req.DateFormat
	}
	if req.Name != nil && *req.Name != "" {
		org.Name = *req.Name
	}

	if err := a.DB.Save(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update settings", nil, "")
	}

	return r.SendEnvelope(map[string]interface{}{
		"message": "Settings updated successfully",
	})
}

// MaskPhoneNumber masks a phone number showing only last 4 digits
func MaskPhoneNumber(phone string) string {
	if len(phone) <= 4 {
		return phone
	}
	masked := ""
	for i := 0; i < len(phone)-4; i++ {
		masked += "*"
	}
	return masked + phone[len(phone)-4:]
}

// ShouldMaskPhoneNumbers checks if phone masking is enabled for the organization
func (a *App) ShouldMaskPhoneNumbers(orgID interface{}) bool {
	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return false
	}

	if org.Settings != nil {
		if v, ok := org.Settings["mask_phone_numbers"].(bool); ok {
			return v
		}
	}
	return false
}
