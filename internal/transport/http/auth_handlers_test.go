package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/auth"
	"github.com/Alexandryn/alexandryn/internal/domain"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
	"golang.org/x/time/rate"
)

// In-memory repositories for transport tests
type memUsers struct {
	byID       map[domain.UserID]*domain.User
	byEmail    map[string]*domain.User
	byUsername map[string]*domain.User
}

func newMemUsers() *memUsers {
	return &memUsers{
		byID:       make(map[domain.UserID]*domain.User),
		byEmail:    make(map[string]*domain.User),
		byUsername: make(map[string]*domain.User),
	}
}

func (m *memUsers) FindByID(_ context.Context, id domain.UserID) (*domain.User, error) {
	if u, ok := m.byID[id]; ok {
		return u, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "user not found"}
}

func (m *memUsers) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, u := range m.byID {
		if u.Email() == email {
			return u, nil
		}
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "user not found"}
}

func (m *memUsers) FindByUsername(_ context.Context, username string) (*domain.User, error) {
	for _, u := range m.byID {
		if u.Username() == username {
			return u, nil
		}
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "user not found"}
}

func (m *memUsers) CountUsers(_ context.Context) (int, error) {
	return len(m.byID), nil
}

func (m *memUsers) Save(_ context.Context, u *domain.User) error {
	m.byID[u.ID()] = u
	return nil
}

type memCredentials struct {
	byUser map[domain.UserID]*domain.UserCredentials
}

func newMemCredentials() *memCredentials {
	return &memCredentials{byUser: make(map[domain.UserID]*domain.UserCredentials)}
}

func (m *memCredentials) FindByUserID(_ context.Context, id domain.UserID) (*domain.UserCredentials, error) {
	if c, ok := m.byUser[id]; ok {
		return c, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "credentials not found"}
}

func (m *memCredentials) Save(_ context.Context, c *domain.UserCredentials) error {
	m.byUser[c.UserID()] = c
	return nil
}

type memRefreshTokens struct {
	byHash map[string]*domain.RefreshToken
}

func newMemRefreshTokens() *memRefreshTokens {
	return &memRefreshTokens{byHash: make(map[string]*domain.RefreshToken)}
}

func (m *memRefreshTokens) FindByTokenHash(_ context.Context, hash string) (*domain.RefreshToken, error) {
	if rt, ok := m.byHash[hash]; ok {
		return rt, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "refresh token not found"}
}

func (m *memRefreshTokens) Save(_ context.Context, rt *domain.RefreshToken) error {
	m.byHash[rt.TokenHash()] = rt
	return nil
}

func (m *memRefreshTokens) RevokeAllForUser(_ context.Context, userID domain.UserID, now time.Time) error {
	for _, rt := range m.byHash {
		if rt.UserID() == userID {
			rt.Revoke(now)
		}
	}
	return nil
}

type memMFA struct {
	byUser map[domain.UserID]*domain.TOTPSettings
}

func newMemMFA() *memMFA {
	return &memMFA{byUser: make(map[domain.UserID]*domain.TOTPSettings)}
}

func (m *memMFA) FindByUserID(_ context.Context, id domain.UserID) (*domain.TOTPSettings, error) {
	if s, ok := m.byUser[id]; ok {
		return s, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "MFA not found"}
}

func (m *memMFA) Save(_ context.Context, s *domain.TOTPSettings) error {
	m.byUser[s.UserID()] = s
	return nil
}

func (m *memMFA) Delete(_ context.Context, id domain.UserID) error {
	delete(m.byUser, id)
	return nil
}

type memLibraries struct {
	byID map[domain.LibraryID]*domain.Library
}

func newMemLibraries() *memLibraries {
	l := &memLibraries{byID: make(map[domain.LibraryID]*domain.Library)}
	defLib, _ := domain.NewLibrary(domain.DefaultLibraryID, "Default Library", "", false, time.Now(), time.Now())
	l.byID[domain.DefaultLibraryID] = defLib
	return l
}

func (m *memLibraries) FindByID(_ context.Context, id domain.LibraryID) (*domain.Library, error) {
	if l, ok := m.byID[id]; ok {
		return l, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "library not found"}
}

func (m *memLibraries) FindAll(_ context.Context) ([]*domain.Library, error) {
	var out []*domain.Library
	for _, l := range m.byID {
		out = append(out, l)
	}
	return out, nil
}

func (m *memLibraries) FindByUser(ctx context.Context, _ domain.UserID) ([]*domain.Library, error) {
	return m.FindAll(ctx)
}

func (m *memLibraries) Save(_ context.Context, l *domain.Library) error {
	m.byID[l.ID()] = l
	return nil
}

func (m *memLibraries) Delete(_ context.Context, id domain.LibraryID) error {
	delete(m.byID, id)
	return nil
}

type memMemberships struct {
	byLibUser map[string]*domain.LibraryMembership
}

func newMemMemberships() *memMemberships {
	return &memMemberships{byLibUser: make(map[string]*domain.LibraryMembership)}
}

func (m *memMemberships) FindMembership(_ context.Context, libID domain.LibraryID, uID domain.UserID) (*domain.LibraryMembership, error) {
	key := string(libID) + ":" + string(uID)
	if mem, ok := m.byLibUser[key]; ok {
		return mem, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "membership not found"}
}

func (m *memMemberships) FindByLibrary(_ context.Context, libID domain.LibraryID) ([]*domain.LibraryMembership, error) {
	var out []*domain.LibraryMembership
	for _, mem := range m.byLibUser {
		if mem.LibraryID() == libID {
			out = append(out, mem)
		}
	}
	return out, nil
}

func (m *memMemberships) FindByUser(_ context.Context, uID domain.UserID) ([]*domain.LibraryMembership, error) {
	var out []*domain.LibraryMembership
	for _, mem := range m.byLibUser {
		if mem.UserID() == uID {
			out = append(out, mem)
		}
	}
	return out, nil
}

func (m *memMemberships) Save(_ context.Context, mem *domain.LibraryMembership) error {
	key := string(mem.LibraryID()) + ":" + string(mem.UserID())
	m.byLibUser[key] = mem
	return nil
}

func (m *memMemberships) Delete(_ context.Context, libID domain.LibraryID, uID domain.UserID) error {
	key := string(libID) + ":" + string(uID)
	delete(m.byLibUser, key)
	return nil
}

func TestAuthSetupStatusAndBootstrap(t *testing.T) {
	users := newMemUsers()
	creds := newMemCredentials()
	libs := newMemLibraries()
	mems := newMemMemberships()
	rtRepo := newMemRefreshTokens()
	hasher := auth.NewArgon2idPasswordHasher(auth.FastArgon2idParamsForTesting())
	signer := auth.NewJWTSigner([]byte("32-byte-test-jwt-secret-key-12345!"), "alexandryn")
	idGen := &seqID{}

	statusHandler := transporthttp.SetupStatusHandler(users)
	setupHandler := transporthttp.SetupHandler(users, creds, libs, mems, rtRepo, hasher, signer, idGen)

	t.Run("initially setup status is false", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/setup/status", nil)
		rec := httptest.NewRecorder()

		statusHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var res struct {
			IsSetup bool `json:"isSetup"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&res)
		if res.IsSetup {
			t.Error("expected isSetup to be false")
		}
	})

	t.Run("setup registers admin account", func(t *testing.T) {
		body := map[string]string{
			"username": "masteradmin",
			"email":    "admin@alexandryn.local",
			"password": "Password123!",
		}
		buf, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/auth/setup", bytes.NewReader(buf))
		rec := httptest.NewRecorder()

		setupHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			User struct {
				ID       string `json:"id"`
				Username string `json:"username"`
				Role     string `json:"role"`
			} `json:"user"`
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&res)
		if res.User.Username != "masteradmin" || res.User.Role != "admin" {
			t.Errorf("unexpected user summary: %+v", res.User)
		}
		if res.AccessToken == "" || res.RefreshToken == "" {
			t.Error("expected access and refresh tokens")
		}
	})

	t.Run("subsequent setup call fails with 409 Conflict", func(t *testing.T) {
		body := map[string]string{
			"username": "secondadmin",
			"email":    "second@alexandryn.local",
			"password": "Password123!",
		}
		buf, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/auth/setup", bytes.NewReader(buf))
		rec := httptest.NewRecorder()

		setupHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict, got %d", rec.Code)
		}
	})
}

func TestAuthLoginAndRefresh(t *testing.T) {
	users := newMemUsers()
	creds := newMemCredentials()
	mfaRepo := newMemMFA()
	mems := newMemMemberships()
	rtRepo := newMemRefreshTokens()
	hasher := auth.NewArgon2idPasswordHasher(auth.FastArgon2idParamsForTesting())
	signer := auth.NewJWTSigner([]byte("32-byte-test-jwt-secret-key-12345!"), "alexandryn")
	idGen := &seqID{}
	limiter := auth.NewIPRateLimiter(rate.Inf, 100, time.Hour)

	// Create user
	u, _ := domain.NewUser("u-1", "testuser", "test@example.com", domain.RoleReader, time.Now(), time.Now())
	_ = users.Save(context.Background(), u)
	pwdHash, _ := hasher.HashPassword("CorrectPassword123!")
	cred, _ := domain.NewUserCredentials("u-1", pwdHash, time.Now())
	_ = creds.Save(context.Background(), cred)

	loginHandler := transporthttp.LoginHandler(users, creds, mfaRepo, mems, rtRepo, hasher, signer, idGen, limiter)
	refreshHandler := transporthttp.RefreshHandler(rtRepo, users, mems, signer, idGen, limiter)
	logoutHandler := transporthttp.LogoutHandler(rtRepo)

	var refreshToken string

	t.Run("successful login", func(t *testing.T) {
		body := map[string]string{
			"emailOrUsername": "test@example.com",
			"password":        "CorrectPassword123!",
		}
		buf, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(buf))
		rec := httptest.NewRecorder()

		loginHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&res)
		if res.AccessToken == "" || res.RefreshToken == "" {
			t.Fatal("expected tokens")
		}
		refreshToken = res.RefreshToken
	})

	t.Run("wrong password fails 401", func(t *testing.T) {
		body := map[string]string{
			"emailOrUsername": "test@example.com",
			"password":        "WrongPassword!",
		}
		buf, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(buf))
		rec := httptest.NewRecorder()

		loginHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("token refresh rotates token", func(t *testing.T) {
		body := map[string]string{"refreshToken": refreshToken}
		buf, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewReader(buf))
		rec := httptest.NewRecorder()

		refreshHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&res)
		if res.RefreshToken == "" || res.RefreshToken == refreshToken {
			t.Error("expected new rotated refresh token")
		}
		refreshToken = res.RefreshToken
	})

	t.Run("logout revokes refresh token", func(t *testing.T) {
		body := map[string]string{"refreshToken": refreshToken}
		buf, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/auth/logout", bytes.NewReader(buf))
		rec := httptest.NewRecorder()

		logoutHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204 No Content, got %d", rec.Code)
		}

		// Subsequent refresh with revoked token fails
		reqRefresh := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewReader(buf))
		recRefresh := httptest.NewRecorder()
		refreshHandler.ServeHTTP(recRefresh, reqRefresh)
		if recRefresh.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 after logout, got %d", recRefresh.Code)
		}
	})
}

// spyHasher wraps a real PasswordHasher and records the encodedHash
// argument of every VerifyPassword call.
type spyHasher struct {
	auth.PasswordHasher
	verifyHashes []string
}

func (s *spyHasher) VerifyPassword(password, encodedHash string) (bool, error) {
	s.verifyHashes = append(s.verifyHashes, encodedHash)
	return s.PasswordHasher.VerifyPassword(password, encodedHash)
}

// #187: a login attempt for a non-existent account must still run the
// password KDF so its response time is indistinguishable from an
// attempt against a real account with a wrong password.
func TestLogin_UnknownUser_StillVerifiesPassword(t *testing.T) {
	spy := &spyHasher{PasswordHasher: auth.NewArgon2idPasswordHasher(auth.FastArgon2idParamsForTesting())}
	limiter := auth.NewIPRateLimiter(rate.Inf, 100, time.Hour)
	h := transporthttp.LoginHandler(
		newMemUsers(), newMemCredentials(), newMemMFA(), newMemMemberships(),
		newMemRefreshTokens(), spy,
		auth.NewJWTSigner([]byte("32-byte-test-jwt-secret-key-12345!"), "alexandryn"),
		&seqID{}, limiter,
	)

	body, _ := json.Marshal(map[string]string{"emailOrUsername": "ghost@example.com", "password": "whatever-123"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body)))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if len(spy.verifyHashes) != 1 {
		t.Fatalf("VerifyPassword called %d times, want 1", len(spy.verifyHashes))
	}
	if spy.verifyHashes[0] != auth.DummyPasswordHash() {
		t.Errorf("VerifyPassword called with %q, want the dummy timing hash", spy.verifyHashes[0])
	}
}

// audit 0016 #107: TOTPSetupHandler must require the account password —
// an access token alone must not be able to overwrite a user's MFA
// enrolment with an attacker-chosen secret.
func TestTOTPSetupHandler_RequiresPassword(t *testing.T) {
	creds := newMemCredentials()
	mfaRepo := newMemMFA()
	hasher := auth.NewArgon2idPasswordHasher(auth.FastArgon2idParamsForTesting())
	totpEngine := auth.NewTOTPEngine("Alexandryn")
	masterKey := bytes.Repeat([]byte("k"), 32)

	pwdHash, _ := hasher.HashPassword("CorrectPassword123!")
	cred, _ := domain.NewUserCredentials("u-1", pwdHash, time.Now())
	_ = creds.Save(context.Background(), cred)

	h := transporthttp.TOTPSetupHandler(mfaRepo, creds, hasher, totpEngine, masterKey)
	call := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/v1/auth/mfa/totp/setup", bytes.NewReader([]byte(body)))
		ctx := transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{UserID: "u-1", Username: "testuser"})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req.WithContext(ctx))
		return rec
	}

	if rec := call(`{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing password: status %d, want 400", rec.Code)
	}
	if rec := call(`{"password":"wrong-password"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: status %d, want 401", rec.Code)
	}
	if _, saved := mfaRepo.byUser["u-1"]; saved {
		t.Fatal("MFA settings were written despite a failed password check")
	}

	rec := call(`{"password":"CorrectPassword123!"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("correct password: status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if _, saved := mfaRepo.byUser["u-1"]; !saved {
		t.Fatal("MFA settings not saved after a successful, password-verified setup")
	}
}
