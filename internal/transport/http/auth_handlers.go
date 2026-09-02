package http

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

type UserSummaryWire struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

type AuthResponseWire struct {
	User         *UserSummaryWire `json:"user,omitempty"`
	AccessToken  string           `json:"accessToken,omitempty"`
	RefreshToken string           `json:"refreshToken,omitempty"`
	MFARequired  bool             `json:"mfaRequired,omitempty"`
	MFATicket    string           `json:"mfaTicket,omitempty"`
}

func clientIP(r *http.Request) string {
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	return ip
}

func generateRandomToken(bytesLen int) (raw string, hash string, err error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])
	return raw, hash, nil
}

// SetupStatusHandler reports whether initial admin setup has been completed (FR-1).
func SetupStatusHandler(userRepo domain.UserRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		count, err := userRepo.CountUsers(r.Context())
		if err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to check setup status", corrID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"isSetup": count > 0})
	})
}

// SetupHandler handles initial admin registration wizard (FR-1).
func SetupHandler(
	userRepo domain.UserRepository,
	credRepo domain.CredentialRepository,
	libRepo domain.LibraryRepository,
	memRepo domain.LibraryMembershipRepository,
	rtRepo domain.RefreshTokenRepository,
	hasher auth.PasswordHasher,
	signer auth.TokenSigner,
	idGen domain.IDGenerator,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())

		count, err := userRepo.CountUsers(r.Context())
		if err != nil {
			WriteError(w, domain.CategoryOf(err), "setup check failed", corrID)
			return
		}
		if count > 0 {
			WriteError(w, domain.Conflict, "system already initialized", corrID)
			return
		}

		var req struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "malformed request payload", corrID)
			return
		}

		if len(req.Password) < 8 || len(req.Password) > 128 {
			WriteError(w, domain.InvalidInput, "password must be between 8 and 128 characters", corrID)
			return
		}

		now := time.Now()
		userID := domain.UserID(idGen.NewID())
		user, err := domain.NewUser(userID, req.Username, req.Email, domain.RoleAdmin, now, now)
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), corrID)
			return
		}

		pwdHash, err := hasher.HashPassword(req.Password)
		if err != nil {
			WriteError(w, domain.Internal, "failed to hash password", corrID)
			return
		}
		creds, err := domain.NewUserCredentials(userID, pwdHash, now)
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), corrID)
			return
		}

		if err := userRepo.Save(r.Context(), user); err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to create user", corrID)
			return
		}
		if err := credRepo.Save(r.Context(), creds); err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to save credentials", corrID)
			return
		}

		// Ensure default library membership
		memID := domain.LibraryMembershipID(idGen.NewID())
		mem, _ := domain.NewLibraryMembership(memID, domain.DefaultLibraryID, userID, domain.RoleAdmin, now)
		_ = memRepo.Save(r.Context(), mem)

		// Create Refresh Token
		rawRT, hashRT, _ := generateRandomToken(32)
		rtID := domain.RefreshTokenID(idGen.NewID())
		rt, _ := domain.NewRefreshToken(rtID, userID, hashRT, now.Add(30*24*time.Hour), now)
		_ = rtRepo.Save(r.Context(), rt)

		// Create Access Token
		accessToken, _ := signer.Sign(auth.Claims{
			Subject:   userID,
			Username:  user.Username(),
			Role:      domain.RoleAdmin,
			Libraries: []domain.LibraryID{domain.DefaultLibraryID},
			JTI:       idGen.NewID(),
			IssuedAt:  now.Unix(),
			ExpiresAt: now.Add(15 * time.Minute).Unix(),
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(AuthResponseWire{
			User: &UserSummaryWire{
				ID:       string(user.ID()),
				Username: user.Username(),
				Email:    user.Email(),
				Role:     string(user.Role()),
			},
			AccessToken:  accessToken,
			RefreshToken: rawRT,
		})
	})
}

// LoginHandler authenticates users with Argon2id and issues tokens (FR-3).
func LoginHandler(
	userRepo domain.UserRepository,
	credRepo domain.CredentialRepository,
	mfaRepo domain.MFARepository,
	memRepo domain.LibraryMembershipRepository,
	rtRepo domain.RefreshTokenRepository,
	hasher auth.PasswordHasher,
	signer auth.TokenSigner,
	idGen domain.IDGenerator,
	limiter *auth.IPRateLimiter,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())

		if limiter != nil && !limiter.Allow(clientIP(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(errorBody{
				Code:          "TooManyRequests",
				Message:       "too many login attempts, please try again later",
				CorrelationID: corrID,
			})
			return
		}

		var req struct {
			EmailOrUsername string `json:"emailOrUsername"`
			Password        string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "malformed request", corrID)
			return
		}

		// Lookup by email or username
		user, err := userRepo.FindByEmail(r.Context(), req.EmailOrUsername)
		if err != nil {
			user, err = userRepo.FindByUsername(r.Context(), req.EmailOrUsername)
			if err != nil {
				WriteError(w, domain.Unauthorized, "invalid credentials", corrID)
				return
			}
		}

		creds, err := credRepo.FindByUserID(r.Context(), user.ID())
		if err != nil {
			WriteError(w, domain.Unauthorized, "invalid credentials", corrID)
			return
		}

		match, err := hasher.VerifyPassword(req.Password, creds.PasswordHash())
		if err != nil || !match {
			WriteError(w, domain.Unauthorized, "invalid credentials", corrID)
			return
		}

		now := time.Now()

		// Check TOTP MFA
		totp, err := mfaRepo.FindByUserID(r.Context(), user.ID())
		if err == nil && totp.IsEnabled() {
			mfaTicket, err := signer.SignMFATicket(user.ID(), now.Add(5*time.Minute))
			if err != nil {
				WriteError(w, domain.Internal, "failed to issue MFA ticket", corrID)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(AuthResponseWire{
				MFARequired: true,
				MFATicket:   mfaTicket,
			})
			return
		}

		// Resolve library memberships for JWT claims
		mems, _ := memRepo.FindByUser(r.Context(), user.ID())
		var libIDs []domain.LibraryID
		for _, m := range mems {
			libIDs = append(libIDs, m.LibraryID())
		}
		if len(libIDs) == 0 {
			libIDs = []domain.LibraryID{domain.DefaultLibraryID}
		}

		rawRT, hashRT, _ := generateRandomToken(32)
		rtID := domain.RefreshTokenID(idGen.NewID())
		rt, _ := domain.NewRefreshToken(rtID, user.ID(), hashRT, now.Add(30*24*time.Hour), now)
		_ = rtRepo.Save(r.Context(), rt)

		accessToken, _ := signer.Sign(auth.Claims{
			Subject:   user.ID(),
			Username:  user.Username(),
			Role:      user.Role(),
			Libraries: libIDs,
			JTI:       idGen.NewID(),
			IssuedAt:  now.Unix(),
			ExpiresAt: now.Add(15 * time.Minute).Unix(),
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AuthResponseWire{
			User: &UserSummaryWire{
				ID:       string(user.ID()),
				Username: user.Username(),
				Email:    user.Email(),
				Role:     string(user.Role()),
			},
			AccessToken:  accessToken,
			RefreshToken: rawRT,
		})
	})
}

// RefreshHandler rotates refresh tokens and issues fresh access tokens (FR-4).
func RefreshHandler(
	rtRepo domain.RefreshTokenRepository,
	userRepo domain.UserRepository,
	memRepo domain.LibraryMembershipRepository,
	signer auth.TokenSigner,
	idGen domain.IDGenerator,
	limiter *auth.IPRateLimiter,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())

		if limiter != nil && !limiter.Allow(clientIP(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(errorBody{
				Code:          "TooManyRequests",
				Message:       "rate limit exceeded",
				CorrelationID: corrID,
			})
			return
		}

		var req struct {
			RefreshToken string `json:"refreshToken"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
			WriteError(w, domain.Unauthorized, "invalid refresh token", corrID)
			return
		}

		h := sha256.Sum256([]byte(req.RefreshToken))
		hash := hex.EncodeToString(h[:])

		now := time.Now()
		rt, err := rtRepo.FindByTokenHash(r.Context(), hash)
		if err != nil || !rt.IsActive(now) {
			WriteError(w, domain.Unauthorized, "invalid or expired refresh token", corrID)
			return
		}

		user, err := userRepo.FindByID(r.Context(), rt.UserID())
		if err != nil {
			WriteError(w, domain.Unauthorized, "user no longer exists", corrID)
			return
		}

		// Rotate: revoke old token, insert new token
		rt.Revoke(now)
		_ = rtRepo.Save(r.Context(), rt)

		rawNewRT, hashNewRT, _ := generateRandomToken(32)
		newRTID := domain.RefreshTokenID(idGen.NewID())
		newRT, _ := domain.NewRefreshToken(newRTID, user.ID(), hashNewRT, now.Add(30*24*time.Hour), now)
		_ = rtRepo.Save(r.Context(), newRT)

		mems, _ := memRepo.FindByUser(r.Context(), user.ID())
		var libIDs []domain.LibraryID
		for _, m := range mems {
			libIDs = append(libIDs, m.LibraryID())
		}
		if len(libIDs) == 0 {
			libIDs = []domain.LibraryID{domain.DefaultLibraryID}
		}

		accessToken, _ := signer.Sign(auth.Claims{
			Subject:   user.ID(),
			Username:  user.Username(),
			Role:      user.Role(),
			Libraries: libIDs,
			JTI:       idGen.NewID(),
			IssuedAt:  now.Unix(),
			ExpiresAt: now.Add(15 * time.Minute).Unix(),
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AuthResponseWire{
			AccessToken:  accessToken,
			RefreshToken: rawNewRT,
		})
	})
}

// LogoutHandler revokes the active refresh token (FR-5).
func LogoutHandler(rtRepo domain.RefreshTokenRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RefreshToken string `json:"refreshToken"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.RefreshToken != "" {
			h := sha256.Sum256([]byte(req.RefreshToken))
			hash := hex.EncodeToString(h[:])
			if rt, err := rtRepo.FindByTokenHash(r.Context(), hash); err == nil {
				rt.Revoke(time.Now())
				_ = rtRepo.Save(r.Context(), rt)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// PasswordResetRequestHandler generates a reset token without revealing account presence (FR-6).
func PasswordResetRequestHandler(
	userRepo domain.UserRepository,
	prRepo domain.PasswordResetRepository,
	idGen domain.IDGenerator,
	limiter *auth.IPRateLimiter,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())

		if limiter != nil && !limiter.Allow(clientIP(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(errorBody{
				Code:          "TooManyRequests",
				Message:       "rate limit exceeded",
				CorrelationID: corrID,
			})
			return
		}

		var req struct {
			Email string `json:"email"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.Email != "" {
			if user, err := userRepo.FindByEmail(r.Context(), req.Email); err == nil {
				_, hash, _ := generateRandomToken(32)
				now := time.Now()
				prt, _ := domain.NewPasswordResetToken(
					domain.PasswordResetID(idGen.NewID()),
					user.ID(),
					hash,
					now.Add(1*time.Hour),
					now,
				)
				_ = prRepo.Save(r.Context(), prt)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "If the account exists, a reset link has been dispatched",
		})
	})
}

// PasswordResetConfirmHandler updates password and revokes existing sessions (FR-6).
func PasswordResetConfirmHandler(
	userRepo domain.UserRepository,
	credRepo domain.CredentialRepository,
	prRepo domain.PasswordResetRepository,
	rtRepo domain.RefreshTokenRepository,
	hasher auth.PasswordHasher,
	limiter *auth.IPRateLimiter,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())

		if limiter != nil && !limiter.Allow(clientIP(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(errorBody{
				Code:          "TooManyRequests",
				Message:       "rate limit exceeded",
				CorrelationID: corrID,
			})
			return
		}

		var req struct {
			Token       string `json:"token"`
			NewPassword string `json:"newPassword"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
			WriteError(w, domain.InvalidInput, "invalid request", corrID)
			return
		}

		if len(req.NewPassword) < 8 || len(req.NewPassword) > 128 {
			WriteError(w, domain.InvalidInput, "password must be between 8 and 128 characters", corrID)
			return
		}

		h := sha256.Sum256([]byte(req.Token))
		hash := hex.EncodeToString(h[:])

		now := time.Now()
		prt, err := prRepo.FindByTokenHash(r.Context(), hash)
		if err != nil || !prt.IsValid(now) {
			WriteError(w, domain.Unauthorized, "invalid or expired reset token", corrID)
			return
		}

		newHash, err := hasher.HashPassword(req.NewPassword)
		if err != nil {
			WriteError(w, domain.Internal, "failed to hash password", corrID)
			return
		}

		creds, _ := domain.NewUserCredentials(prt.UserID(), newHash, now)
		if err := credRepo.Save(r.Context(), creds); err != nil {
			WriteError(w, domain.Internal, "failed to save credentials", corrID)
			return
		}

		prt.MarkUsed(now)
		_ = prRepo.Save(r.Context(), prt)
		_ = rtRepo.RevokeAllForUser(r.Context(), prt.UserID(), now)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
	})
}

// TOTPSetupHandler starts 2FA enrollment (ADR 0027).
func TOTPSetupHandler(mfaRepo domain.MFARepository, totpEngine *auth.TOTPEngine, masterKey []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}

		secret, err := totpEngine.GenerateSecret()
		if err != nil {
			WriteError(w, domain.Internal, "failed to generate TOTP secret", corrID)
			return
		}

		codes, hashes, err := totpEngine.GenerateRecoveryCodes(8)
		if err != nil {
			WriteError(w, domain.Internal, "failed to generate recovery codes", corrID)
			return
		}

		encryptedSecret, err := auth.EncryptSecret([]byte(secret), masterKey)
		if err != nil {
			WriteError(w, domain.Internal, "failed to encrypt secret", corrID)
			return
		}

		now := time.Now()
		settings, err := domain.NewTOTPSettings(user.UserID, encryptedSecret, hashes, false, nil, now)
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), corrID)
			return
		}

		if err := mfaRepo.Save(r.Context(), settings); err != nil {
			WriteError(w, domain.Internal, "failed to save MFA settings", corrID)
			return
		}

		keyURI := totpEngine.KeyURI(user.Username, secret)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"secret":        secret,
			"keyUri":        keyURI,
			"recoveryCodes": codes,
		})
	})
}

// TOTPConfirmHandler confirms 2FA enrollment with first code (ADR 0027).
func TOTPConfirmHandler(mfaRepo domain.MFARepository, totpEngine *auth.TOTPEngine, masterKey []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}

		var req struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
			WriteError(w, domain.InvalidInput, "missing code", corrID)
			return
		}

		settings, err := mfaRepo.FindByUserID(r.Context(), user.UserID)
		if err != nil {
			WriteError(w, domain.NotFound, "MFA setup not initiated", corrID)
			return
		}

		secretBytes, err := auth.DecryptSecret(settings.EncryptedSecret(), masterKey)
		if err != nil {
			WriteError(w, domain.Internal, "failed to decrypt secret", corrID)
			return
		}

		now := time.Now()
		valid, err := totpEngine.ValidateCode(string(secretBytes), req.Code, now)
		if err != nil || !valid {
			WriteError(w, domain.InvalidInput, "invalid TOTP verification code", corrID)
			return
		}

		settings.Confirm(now)
		if err := mfaRepo.Save(r.Context(), settings); err != nil {
			WriteError(w, domain.Internal, "failed to activate MFA", corrID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"enabled": true})
	})
}

// TOTPVerifyHandler verifies MFA during login (ADR 0027).
func TOTPVerifyHandler(
	mfaRepo domain.MFARepository,
	userRepo domain.UserRepository,
	rtRepo domain.RefreshTokenRepository,
	memRepo domain.LibraryMembershipRepository,
	totpEngine *auth.TOTPEngine,
	signer auth.TokenSigner,
	idGen domain.IDGenerator,
	masterKey []byte,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())

		var req struct {
			MFATicket    string `json:"mfaTicket"`
			Code         string `json:"code,omitempty"`
			RecoveryCode string `json:"recoveryCode,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MFATicket == "" {
			WriteError(w, domain.InvalidInput, "missing MFA ticket", corrID)
			return
		}

		now := time.Now()
		userID, err := signer.VerifyMFATicket(req.MFATicket, now)
		if err != nil {
			WriteError(w, domain.Unauthorized, "invalid or expired MFA ticket", corrID)
			return
		}

		settings, err := mfaRepo.FindByUserID(r.Context(), userID)
		if err != nil || !settings.IsEnabled() {
			WriteError(w, domain.Unauthorized, "MFA not enabled for user", corrID)
			return
		}

		user, err := userRepo.FindByID(r.Context(), userID)
		if err != nil {
			WriteError(w, domain.Unauthorized, "user not found", corrID)
			return
		}

		verified := false

		if req.Code != "" {
			secretBytes, err := auth.DecryptSecret(settings.EncryptedSecret(), masterKey)
			if err == nil {
				valid, _ := totpEngine.ValidateCode(string(secretBytes), req.Code, now)
				verified = valid
			}
		} else if req.RecoveryCode != "" {
			matchIdx := totpEngine.VerifyRecoveryCode(req.RecoveryCode, settings.RecoveryCodeHashes())
			if matchIdx != -1 {
				verified = true
				// Invalidate the used recovery code
				hashes := settings.RecoveryCodeHashes()
				newHashes := append(hashes[:matchIdx], hashes[matchIdx+1:]...)
				newSettings, _ := domain.NewTOTPSettings(userID, settings.EncryptedSecret(), newHashes, true, settings.ConfirmedAt(), settings.CreatedAt())
				_ = mfaRepo.Save(r.Context(), newSettings)
			}
		}

		if !verified {
			WriteError(w, domain.Unauthorized, "invalid MFA code or recovery code", corrID)
			return
		}

		mems, _ := memRepo.FindByUser(r.Context(), user.ID())
		var libIDs []domain.LibraryID
		for _, m := range mems {
			libIDs = append(libIDs, m.LibraryID())
		}
		if len(libIDs) == 0 {
			libIDs = []domain.LibraryID{domain.DefaultLibraryID}
		}

		rawRT, hashRT, _ := generateRandomToken(32)
		rtID := domain.RefreshTokenID(idGen.NewID())
		rt, _ := domain.NewRefreshToken(rtID, user.ID(), hashRT, now.Add(30*24*time.Hour), now)
		_ = rtRepo.Save(r.Context(), rt)

		accessToken, _ := signer.Sign(auth.Claims{
			Subject:   user.ID(),
			Username:  user.Username(),
			Role:      user.Role(),
			Libraries: libIDs,
			JTI:       idGen.NewID(),
			IssuedAt:  now.Unix(),
			ExpiresAt: now.Add(15 * time.Minute).Unix(),
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AuthResponseWire{
			User: &UserSummaryWire{
				ID:       string(user.ID()),
				Username: user.Username(),
				Email:    user.Email(),
				Role:     string(user.Role()),
			},
			AccessToken:  accessToken,
			RefreshToken: rawRT,
		})
	})
}

// TOTPDisableHandler disables 2FA upon password confirmation.
func TOTPDisableHandler(mfaRepo domain.MFARepository, credRepo domain.CredentialRepository, hasher auth.PasswordHasher) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}

		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Password == "" {
			WriteError(w, domain.InvalidInput, "password required to disable 2FA", corrID)
			return
		}

		creds, err := credRepo.FindByUserID(r.Context(), user.UserID)
		if err != nil {
			WriteError(w, domain.Unauthorized, "credentials not found", corrID)
			return
		}

		match, err := hasher.VerifyPassword(req.Password, creds.PasswordHash())
		if err != nil || !match {
			WriteError(w, domain.Unauthorized, "invalid password", corrID)
			return
		}

		if err := mfaRepo.Delete(r.Context(), user.UserID); err != nil {
			WriteError(w, domain.Internal, "failed to delete MFA settings", corrID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"disabled": true})
	})
}
