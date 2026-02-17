package handlers

import (
	"crypto/subtle"
	"encoding/base64"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
	"github.com/shridarpatil/whatomate/internal/middleware"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"golang.org/x/crypto/bcrypt"
)

const (
	totpStepSeconds   = 30
	twoFATokenExpiry  = 5 * time.Minute
	twoFATokenPurpose = "two_fa_login"
	twoFASetupPurpose = "two_fa_setup"
)

type TwoFAClaims struct {
	UserID  uuid.UUID `json:"user_id"`
	Purpose string    `json:"purpose"`
	jwt.RegisteredClaims
}

type TOTPSetupResponse struct {
	Secret    string `json:"secret"`
	OTPAuth   string `json:"otpauth_url"`
	QRCodePNG string `json:"qr_code"` // data URL
}

type TOTPVerifyRequest struct {
	Code string `json:"code" validate:"required"`
}

type TOTPDisableRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
}

type TOTPResetRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
}

type TwoFAVerifyRequest struct {
	TwoFAToken string `json:"two_fa_token" validate:"required"`
	Code       string `json:"code" validate:"required"`
}

// SetupTOTP initializes a TOTP secret for the current user.
func (a *App) SetupTOTP(r *fastglue.Request) error {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "User not found", nil, "")
	}

	if user.TOTPEnabled {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "Two-factor authentication is already enabled", nil, "")
	}

	return a.generateAndStoreTOTPSecret(r, &user)
}

// VerifyTOTP enables TOTP for the current user after code verification.
func (a *App) VerifyTOTP(r *fastglue.Request) error {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req TOTPVerifyRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "User not found", nil, "")
	}

	if user.TOTPSecret == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Two-factor authentication is not setup", nil, "")
	}

	if ok, usedAt := validateTOTPCode(user.TOTPSecret, req.Code, time.Now().UTC(), user.TOTPLastUsedAt); !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid verification code", nil, "")
	} else {
		if err := a.DB.Model(&user).Updates(map[string]any{
			"totp_enabled":      true,
			"totp_last_used_at": usedAt,
		}).Error; err != nil {
			a.Log.Error("Failed to enable TOTP", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to enable TOTP", nil, "")
		}
	}

	return r.SendEnvelope(map[string]any{
		"message":      "Two-factor authentication enabled",
		"totp_enabled": true,
	})
}

// DisableTOTP disables TOTP for the current user.
func (a *App) DisableTOTP(r *fastglue.Request) error {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req TOTPDisableRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "User not found", nil, "")
	}

	if !user.TOTPEnabled || user.TOTPSecret == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Two-factor authentication is not enabled", nil, "")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid password", nil, "")
	}

	if err := a.DB.Model(&user).Updates(map[string]any{
		"totp_secret":       "",
		"totp_enabled":      false,
		"totp_last_used_at": nil,
	}).Error; err != nil {
		a.Log.Error("Failed to disable TOTP", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to disable TOTP", nil, "")
	}

	return r.SendEnvelope(map[string]any{
		"message":      "Two-factor authentication disabled",
		"totp_enabled": false,
	})
}

// ResetTOTP rotates the TOTP secret after password verification.
func (a *App) ResetTOTP(r *fastglue.Request) error {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req TOTPResetRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "User not found", nil, "")
	}

	if !user.TOTPEnabled || user.TOTPSecret == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Two-factor authentication is not enabled", nil, "")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid password", nil, "")
	}

	return a.generateAndStoreTOTPSecret(r, &user)
}

// VerifyTwoFALogin exchanges a valid TOTP code for full auth tokens.
func (a *App) VerifyTwoFALogin(r *fastglue.Request) error {
	var req TwoFAVerifyRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	token, err := jwt.ParseWithClaims(req.TwoFAToken, &TwoFAClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(a.Config.JWT.Secret), nil
	})
	if err != nil || !token.Valid {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid or expired 2FA token", nil, "")
	}

	claims, ok := token.Claims.(*TwoFAClaims)
	if !ok || (claims.Purpose != twoFATokenPurpose && claims.Purpose != twoFASetupPurpose) {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid 2FA token", nil, "")
	}

	var user models.User
	if err := a.DB.Preload("Role").Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "User not found", nil, "")
	}

	// Load permissions from cache
	if user.Role != nil && user.RoleID != nil {
		cachedPerms, err := a.GetRolePermissionsCached(*user.RoleID)
		if err == nil {
			permissions := make([]models.Permission, 0, len(cachedPerms))
			for _, p := range cachedPerms {
				for i := len(p) - 1; i >= 0; i-- {
					if p[i] == ':' {
						permissions = append(permissions, models.Permission{
							Resource: p[:i],
							Action:   p[i+1:],
						})
						break
					}
				}
			}
			user.Role.Permissions = permissions
		}
	}

	if !user.IsActive {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Account is disabled", nil, "")
	}

	if user.TOTPSecret == "" {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Two-factor authentication is not setup", nil, "")
	}
	if claims.Purpose == twoFATokenPurpose && !user.TOTPEnabled {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Two-factor authentication is not enabled", nil, "")
	}

	okCode, usedAt := validateTOTPCode(user.TOTPSecret, req.Code, time.Now().UTC(), user.TOTPLastUsedAt)
	if !okCode {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid verification code", nil, "")
	}

	updates := map[string]any{
		"totp_last_used_at": usedAt,
	}
	if claims.Purpose == twoFASetupPurpose && !user.TOTPEnabled {
		updates["totp_enabled"] = true
	}
	if err := a.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to update TOTP last used time", "error", err)
	}

	// Reload user to reflect updated TOTP state for the response
	if err := a.DB.Preload("Role").Where("id = ?", user.ID).First(&user).Error; err == nil {
		if user.Role != nil && user.RoleID != nil {
			cachedPerms, err := a.GetRolePermissionsCached(*user.RoleID)
			if err == nil {
				permissions := make([]models.Permission, 0, len(cachedPerms))
				for _, p := range cachedPerms {
					for i := len(p) - 1; i >= 0; i-- {
						if p[i] == ':' {
							permissions = append(permissions, models.Permission{
								Resource: p[:i],
								Action:   p[i+1:],
							})
							break
						}
					}
				}
				user.Role.Permissions = permissions
			}
		}
	}

	accessToken, err := a.generateAccessToken(&user)
	if err != nil {
		a.Log.Error("Failed to generate access token", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to generate token", nil, "")
	}

	refreshToken, err := a.generateRefreshToken(&user)
	if err != nil {
		a.Log.Error("Failed to generate refresh token", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to generate token", nil, "")
	}

	a.setAuthCookies(r, accessToken, refreshToken)

	return r.SendEnvelope(CookieAuthResponse{
		ExpiresIn: a.Config.JWT.AccessExpiryMins * 60,
		User:      user,
	})
}

func (a *App) generateTwoFAToken(user *models.User) (string, error) {
	claims := TwoFAClaims{
		UserID:  user.ID,
		Purpose: twoFATokenPurpose,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(twoFATokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "whatomate",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.Config.JWT.Secret))
}

func (a *App) generateTwoFASetupToken(user *models.User) (string, error) {
	claims := TwoFAClaims{
		UserID:  user.ID,
		Purpose: twoFASetupPurpose,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(twoFATokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "whatomate",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.Config.JWT.Secret))
}

// SetupTOTPWithToken initializes a TOTP secret for a user after password login.
func (a *App) SetupTOTPWithToken(r *fastglue.Request) error {
	var req struct {
		TwoFAToken string `json:"two_fa_token" validate:"required"`
	}
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	token, err := jwt.ParseWithClaims(req.TwoFAToken, &TwoFAClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(a.Config.JWT.Secret), nil
	})
	if err != nil || !token.Valid {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid or expired 2FA token", nil, "")
	}

	claims, ok := token.Claims.(*TwoFAClaims)
	if !ok || claims.Purpose != twoFASetupPurpose {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Invalid 2FA token", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "User not found", nil, "")
	}

	if user.TOTPEnabled {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "Two-factor authentication is already enabled", nil, "")
	}

	return a.generateAndStoreTOTPSecret(r, &user)
}

func (a *App) generateAndStoreTOTPSecret(r *fastglue.Request, user *models.User) error {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Whatomate",
		AccountName: user.Email,
	})
	if err != nil {
		a.Log.Error("Failed to generate TOTP key", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to generate TOTP secret", nil, "")
	}

	secret := key.Secret()
	otpauthURL := key.URL()

	if err := a.DB.Model(user).Updates(map[string]any{
		"totp_secret":       secret,
		"totp_enabled":      false,
		"totp_last_used_at": nil,
	}).Error; err != nil {
		a.Log.Error("Failed to store TOTP secret", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to store TOTP secret", nil, "")
	}

	png, err := qrcode.Encode(otpauthURL, qrcode.Medium, 256)
	if err != nil {
		a.Log.Error("Failed to generate TOTP QR code", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to generate QR code", nil, "")
	}

	qrDataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)

	return r.SendEnvelope(TOTPSetupResponse{
		Secret:    secret,
		OTPAuth:   otpauthURL,
		QRCodePNG: qrDataURL,
	})
}

func validateTOTPCode(secret, code string, now time.Time, lastUsedAt *time.Time) (bool, time.Time) {
	cleanCode := strings.TrimSpace(code)
	if cleanCode == "" {
		return false, time.Time{}
	}

	for offset := -1; offset <= 1; offset++ {
		checkTime := now.Add(time.Duration(offset*totpStepSeconds) * time.Second)
		expected, err := totp.GenerateCode(secret, checkTime)
		if err != nil {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(expected), []byte(cleanCode)) == 1 {
			if lastUsedAt != nil {
				lastStep := lastUsedAt.Unix() / totpStepSeconds
				step := checkTime.Unix() / totpStepSeconds
				if step <= lastStep {
					return false, time.Time{}
				}
			}
			return true, checkTime
		}
	}

	return false, time.Time{}
}
