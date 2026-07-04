package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/triasbrata/mihon-manga-server/internal/config"
	"github.com/triasbrata/mihon-manga-server/internal/repository/d1"
	"github.com/triasbrata/mihon-manga-server/internal/repository/kv"
)

// Compile-time interface checks.
var (
	_ op.Storage                  = (*Storage)(nil)
	_ op.ClientCredentialsStorage = (*Storage)(nil)
)

// KV key prefixes for ephemeral OIDC state.
const (
	kvAuthReqPrefix = "oidc:authreq:"
	kvCodePrefix    = "oidc:code:"
	kvTokenPrefix   = "oidc:token:"
)

// Storage implements op.Storage and op.ClientCredentialsStorage over D1
// (durable) and KV (ephemeral).
type Storage struct {
	d1 *d1.Client
	kv *kv.Client

	issuer      string
	accessTTL   time.Duration
	refreshTTL  time.Duration
	authCodeTTL time.Duration
	authReqTTL  time.Duration

	adminEmail    string
	adminPassword string

	signKey *signingKey
}

// signingKey implements op.SigningKey.
type signingKey struct {
	id  string
	alg jose.SignatureAlgorithm
	key *rsa.PrivateKey
}

func (s *signingKey) SignatureAlgorithm() jose.SignatureAlgorithm { return s.alg }
func (s *signingKey) Key() any                                    { return s.key }
func (s *signingKey) ID() string                                  { return s.id }

// publicKey implements op.Key.
type publicKey struct {
	*signingKey
}

func (p *publicKey) ID() string                       { return p.id }
func (p *publicKey) Algorithm() jose.SignatureAlgorithm { return p.alg }
func (p *publicKey) Use() string                      { return "sig" }
func (p *publicKey) Key() any                         { return &p.key.PublicKey }

// NewStorage builds the D1/KV-backed storage, loading or generating the signing
// key and seeding the static admin client + admin user on first boot.
func NewStorage(d1c *d1.Client, kvc *kv.Client, cfg config.OIDCConfig) *Storage {
	s := &Storage{
		d1:            d1c,
		kv:            kvc,
		issuer:        cfg.Issuer,
		accessTTL:     cfg.AccessTokenTTL,
		refreshTTL:    cfg.RefreshTokenTTL,
		authCodeTTL:   cfg.AuthCodeTTL,
		authReqTTL:    30 * time.Minute,
		adminEmail:    cfg.AdminEmail,
		adminPassword: cfg.AdminPassword,
	}
	ctx := context.Background()
	s.loadOrCreateSigningKey(ctx)
	s.seedAdmin(ctx)
	seedAdminClient(ctx, d1c, cfg)
	seedIOSClient(ctx, d1c, cfg)
	return s
}

// loadOrCreateSigningKey loads the active RSA key from D1, or generates one and
// (best-effort) persists it. If D1 is unconfigured/unreachable it falls back to
// an in-memory key so the process can still boot for docs/health.
func (s *Storage) loadOrCreateSigningKey(ctx context.Context) {
	rows, err := s.d1.Query(ctx,
		`SELECT id, private_key FROM oidc_signing_key WHERE active = 1 ORDER BY created_at DESC LIMIT 1`)
	if err == nil && len(rows) > 0 {
		id := strVal(rows[0]["id"])
		if key, perr := parsePKCS8(strVal(rows[0]["private_key"])); perr == nil {
			s.signKey = &signingKey{id: id, alg: jose.RS256, key: key}
			return
		}
		slog.Warn("oidc: stored signing key unparseable, regenerating")
	}

	key, gerr := rsa.GenerateKey(rand.Reader, 2048)
	if gerr != nil {
		slog.Error("oidc: failed to generate signing key", "err", gerr)
		return
	}
	id := uuid.NewString()
	s.signKey = &signingKey{id: id, alg: jose.RS256, key: key}

	pemStr, perr := marshalPKCS8(key)
	if perr != nil {
		slog.Warn("oidc: failed to encode signing key for persistence", "err", perr)
		return
	}
	if err := s.d1.Exec(ctx,
		`INSERT INTO oidc_signing_key (id, algorithm, private_key, active, created_at)
		 VALUES (?1, 'RS256', ?2, 1, ?3)`,
		id, pemStr, time.Now().Unix()); err != nil {
		slog.Warn("oidc: signing key not persisted (running with in-memory key)", "err", err)
		return
	}
	slog.Info("oidc: generated and persisted new signing key", "kid", id)
}

func parsePKCS8(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("invalid PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an RSA private key")
	}
	return key, nil
}

func marshalPKCS8(key *rsa.PrivateKey) (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), nil
}

// --- KV helpers ---

func (s *Storage) putJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.kv.Put(ctx, key, b, ttl)
}

func (s *Storage) getJSON(ctx context.Context, key string, v any) error {
	b, err := s.kv.Get(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// ============================ AuthStorage ============================

func (s *Storage) CreateAuthRequest(ctx context.Context, authReq *oidc.AuthRequest, userID string) (op.AuthRequest, error) {
	if len(authReq.Prompt) == 1 && authReq.Prompt[0] == "none" {
		return nil, oidc.ErrLoginRequired()
	}
	request := authRequestToInternal(authReq, userID)
	request.ID = uuid.NewString()
	if err := s.putJSON(ctx, kvAuthReqPrefix+request.ID, request, s.authReqTTL); err != nil {
		return nil, err
	}
	return request, nil
}

func (s *Storage) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	req := &AuthRequest{}
	if err := s.getJSON(ctx, kvAuthReqPrefix+id, req); err != nil {
		return nil, fmt.Errorf("auth request not found: %w", err)
	}
	return req, nil
}

func (s *Storage) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	b, err := s.kv.Get(ctx, kvCodePrefix+code)
	if err != nil {
		return nil, fmt.Errorf("code invalid or expired: %w", err)
	}
	return s.AuthRequestByID(ctx, string(b))
}

func (s *Storage) SaveAuthCode(ctx context.Context, id string, code string) error {
	return s.kv.Put(ctx, kvCodePrefix+code, []byte(id), s.authCodeTTL)
}

func (s *Storage) DeleteAuthRequest(ctx context.Context, id string) error {
	// Codes expire on their own TTL; deleting the request is enough to prevent
	// replay (AuthRequestByCode re-reads the request by id).
	return s.kv.Delete(ctx, kvAuthReqPrefix+id)
}

func (s *Storage) CreateAccessToken(ctx context.Context, request op.TokenRequest) (string, time.Time, error) {
	applicationID := ""
	switch req := request.(type) {
	case *AuthRequest:
		applicationID = req.ApplicationID
	case op.TokenExchangeRequest:
		applicationID = req.GetClientID()
	}
	token, err := s.accessToken(ctx, applicationID, "", request.GetSubject(), request.GetAudience(), request.GetScopes())
	if err != nil {
		return "", time.Time{}, err
	}
	return token.ID, token.Expiration, nil
}

func (s *Storage) CreateAccessAndRefreshTokens(ctx context.Context, request op.TokenRequest, currentRefreshToken string) (string, string, time.Time, error) {
	applicationID, authTime, amr := infoFromRequest(request)

	if currentRefreshToken == "" {
		// Authorization code flow with offline_access: mint a fresh pair.
		refreshTokenID := uuid.NewString()
		accessToken, err := s.accessToken(ctx, applicationID, refreshTokenID, request.GetSubject(), request.GetAudience(), request.GetScopes())
		if err != nil {
			return "", "", time.Time{}, err
		}
		refreshToken, err := s.createRefreshToken(ctx, accessToken, amr, authTime)
		if err != nil {
			return "", "", time.Time{}, err
		}
		return accessToken.ID, refreshToken, accessToken.Expiration, nil
	}

	// Refresh token request: rotate.
	newRefreshToken := uuid.NewString()
	accessToken, err := s.accessToken(ctx, applicationID, newRefreshToken, request.GetSubject(), request.GetAudience(), request.GetScopes())
	if err != nil {
		return "", "", time.Time{}, err
	}
	if err := s.renewRefreshToken(ctx, currentRefreshToken, newRefreshToken, accessToken.ID); err != nil {
		return "", "", time.Time{}, err
	}
	return accessToken.ID, newRefreshToken, accessToken.Expiration, nil
}

func (s *Storage) TokenRequestByRefreshToken(ctx context.Context, refreshToken string) (op.RefreshTokenRequest, error) {
	rt, err := s.refreshTokenByValue(ctx, refreshToken)
	if err != nil || rt == nil {
		return nil, fmt.Errorf("invalid refresh_token")
	}
	return refreshTokenRequest(rt), nil
}

func (s *Storage) TerminateSession(ctx context.Context, userID string, clientID string) error {
	// Remove durable refresh tokens for this user+client. Access tokens are in
	// KV and expire on their own.
	return s.d1.Exec(ctx,
		`DELETE FROM oidc_refresh_token WHERE user_id = ?1 AND client_id = ?2`, userID, clientID)
}

func (s *Storage) GetRefreshTokenInfo(ctx context.Context, clientID string, token string) (string, string, error) {
	rt, err := s.refreshTokenByValue(ctx, token)
	if err != nil {
		return "", "", err
	}
	if rt == nil {
		return "", "", op.ErrInvalidRefreshToken
	}
	return rt.UserID, rt.ID, nil
}

func (s *Storage) RevokeToken(ctx context.Context, tokenIDOrToken string, userID string, clientID string) *oidc.Error {
	// Access token by ID (in KV).
	tok := &Token{}
	if err := s.getJSON(ctx, kvTokenPrefix+tokenIDOrToken, tok); err == nil {
		if tok.ApplicationID != clientID {
			return oidc.ErrInvalidClient().WithDescription("token was not issued for this client")
		}
		_ = s.kv.Delete(ctx, kvTokenPrefix+tok.ID)
		return nil
	}
	// Otherwise treat as a refresh token value (in D1).
	rt, err := s.refreshTokenByValue(ctx, tokenIDOrToken)
	if err != nil || rt == nil {
		return nil // unknown token: revocation is a no-op success
	}
	if rt.ApplicationID != clientID {
		return oidc.ErrInvalidClient().WithDescription("token was not issued for this client")
	}
	_ = s.d1.Exec(ctx, `DELETE FROM oidc_refresh_token WHERE id = ?1`, rt.ID)
	_ = s.kv.Delete(ctx, kvTokenPrefix+rt.AccessToken)
	return nil
}

func (s *Storage) SigningKey(ctx context.Context) (op.SigningKey, error) {
	if s.signKey == nil {
		return nil, errors.New("no signing key available")
	}
	return s.signKey, nil
}

func (s *Storage) SignatureAlgorithms(context.Context) ([]jose.SignatureAlgorithm, error) {
	return []jose.SignatureAlgorithm{jose.RS256}, nil
}

func (s *Storage) KeySet(ctx context.Context) ([]op.Key, error) {
	if s.signKey == nil {
		return nil, errors.New("no signing key available")
	}
	return []op.Key{&publicKey{s.signKey}}, nil
}

// ============================ OPStorage ============================

func (s *Storage) GetClientByClientID(ctx context.Context, clientID string) (op.Client, error) {
	c, err := s.clientByID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, fmt.Errorf("client not found")
	}
	return c, nil
}

func (s *Storage) AuthorizeClientIDSecret(ctx context.Context, clientID, clientSecret string) error {
	c, err := s.clientByID(ctx, clientID)
	if err != nil {
		return err
	}
	if c == nil {
		return fmt.Errorf("client not found")
	}
	if c.secretHash == "" || !verifyPassword(clientSecret, c.secretHash) {
		return fmt.Errorf("invalid client secret")
	}
	return nil
}

func (s *Storage) SetUserinfoFromScopes(ctx context.Context, userinfo *oidc.UserInfo, userID, clientID string, scopes []string) error {
	return nil
}

// SetUserinfoFromRequest implements op.CanSetUserinfoFromRequest.
func (s *Storage) SetUserinfoFromRequest(ctx context.Context, userinfo *oidc.UserInfo, token op.IDTokenRequest, scopes []string) error {
	return s.setUserinfo(ctx, userinfo, token.GetSubject(), scopes)
}

func (s *Storage) SetUserinfoFromToken(ctx context.Context, userinfo *oidc.UserInfo, tokenID, subject, origin string) error {
	tok := &Token{}
	if err := s.getJSON(ctx, kvTokenPrefix+tokenID, tok); err != nil {
		return fmt.Errorf("token is invalid or has expired")
	}
	if tok.Expiration.Before(time.Now()) {
		return fmt.Errorf("token is expired")
	}
	return s.setUserinfo(ctx, userinfo, tok.Subject, tok.Scopes)
}

func (s *Storage) SetIntrospectionFromToken(ctx context.Context, introspection *oidc.IntrospectionResponse, tokenID, subject, clientID string) error {
	tok := &Token{}
	if err := s.getJSON(ctx, kvTokenPrefix+tokenID, tok); err != nil {
		return fmt.Errorf("token is invalid")
	}
	introspection.Expiration = oidc.FromTime(tok.Expiration)
	if tok.Expiration.Before(time.Now()) {
		return fmt.Errorf("token is expired")
	}
	for _, aud := range tok.Audience {
		if aud == clientID {
			userInfo := new(oidc.UserInfo)
			if err := s.setUserinfo(ctx, userInfo, subject, tok.Scopes); err != nil {
				return err
			}
			introspection.SetUserInfo(userInfo)
			introspection.Scope = tok.Scopes
			introspection.ClientID = tok.ApplicationID
			return nil
		}
	}
	return fmt.Errorf("token is not valid for this client")
}

func (s *Storage) GetPrivateClaimsFromScopes(ctx context.Context, userID, clientID string, scopes []string) (map[string]any, error) {
	return nil, nil
}

func (s *Storage) GetKeyByIDAndClientID(ctx context.Context, keyID, clientID string) (*jose.JSONWebKey, error) {
	// JWT-profile (private_key_jwt) client assertions are not supported.
	return nil, fmt.Errorf("client key not found")
}

func (s *Storage) ValidateJWTProfileScopes(ctx context.Context, userID string, scopes []string) ([]string, error) {
	allowed := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if scope == oidc.ScopeOpenID {
			allowed = append(allowed, scope)
		}
	}
	return allowed, nil
}

// Health implements op.Storage.
func (s *Storage) Health(ctx context.Context) error { return nil }

// ============================ ClientCredentialsStorage ============================

func (s *Storage) ClientCredentials(ctx context.Context, clientID, clientSecret string) (op.Client, error) {
	c, err := s.clientByID(ctx, clientID)
	if err != nil || c == nil {
		return nil, errors.New("wrong service user or password")
	}
	if !hasGrantType(c.grantTypes, oidc.GrantTypeClientCredentials) {
		return nil, errors.New("client not permitted for client_credentials")
	}
	if c.secretHash == "" || !verifyPassword(clientSecret, c.secretHash) {
		return nil, errors.New("wrong service user or password")
	}
	return c, nil
}

func (s *Storage) ClientCredentialsTokenRequest(ctx context.Context, clientID string, scopes []string) (op.TokenRequest, error) {
	c, err := s.clientByID(ctx, clientID)
	if err != nil || c == nil {
		return nil, errors.New("wrong service user or password")
	}
	return &oidc.JWTTokenRequest{
		Subject:  clientID,
		Audience: []string{clientID},
		Scopes:   scopes,
	}, nil
}

// ============================ helpers ============================

func (s *Storage) setUserinfo(ctx context.Context, userInfo *oidc.UserInfo, userID string, scopes []string) error {
	user, err := s.userByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}
	for _, scope := range scopes {
		switch scope {
		case oidc.ScopeOpenID:
			userInfo.Subject = user.ID
		case oidc.ScopeEmail:
			userInfo.Email = user.Email
			userInfo.EmailVerified = oidc.Bool(user.EmailVerified)
		case oidc.ScopeProfile:
			userInfo.Name = user.Name
			userInfo.PreferredUsername = user.Email
		}
	}
	return nil
}

func (s *Storage) accessToken(ctx context.Context, applicationID, refreshTokenID, subject string, audience, scopes []string) (*Token, error) {
	token := &Token{
		ID:             uuid.NewString(),
		ApplicationID:  applicationID,
		RefreshTokenID: refreshTokenID,
		Subject:        subject,
		Audience:       audience,
		Expiration:     time.Now().Add(s.accessTTL),
		Scopes:         scopes,
	}
	if err := s.putJSON(ctx, kvTokenPrefix+token.ID, token, s.accessTTL); err != nil {
		return nil, err
	}
	return token, nil
}

func (s *Storage) createRefreshToken(ctx context.Context, accessToken *Token, amr []string, authTime time.Time) (string, error) {
	rt := &RefreshToken{
		ID:            accessToken.RefreshTokenID,
		Token:         accessToken.RefreshTokenID,
		AuthTime:      authTime,
		AMR:           amr,
		ApplicationID: accessToken.ApplicationID,
		UserID:        accessToken.Subject,
		Audience:      accessToken.Audience,
		Expiration:    time.Now().Add(s.refreshTTL),
		Scopes:        accessToken.Scopes,
		AccessToken:   accessToken.ID,
	}
	if err := s.insertRefreshToken(ctx, rt); err != nil {
		return "", err
	}
	return rt.Token, nil
}

func (s *Storage) renewRefreshToken(ctx context.Context, currentToken, newToken, newAccessToken string) error {
	rt, err := s.refreshTokenByValue(ctx, currentToken)
	if err != nil {
		return err
	}
	if rt == nil {
		return fmt.Errorf("invalid refresh token")
	}
	if rt.Expiration.Before(time.Now()) {
		_ = s.d1.Exec(ctx, `DELETE FROM oidc_refresh_token WHERE id = ?1`, rt.ID)
		return fmt.Errorf("expired refresh token")
	}
	// Delete the old access token and rotate the refresh token (RFC 6819 5.2.2.3).
	_ = s.kv.Delete(ctx, kvTokenPrefix+rt.AccessToken)
	if err := s.d1.Exec(ctx, `DELETE FROM oidc_refresh_token WHERE id = ?1`, rt.ID); err != nil {
		return err
	}
	rt.ID = newToken
	rt.Token = newToken
	rt.Expiration = time.Now().Add(s.refreshTTL)
	rt.AccessToken = newAccessToken
	return s.insertRefreshToken(ctx, rt)
}

func (s *Storage) insertRefreshToken(ctx context.Context, rt *RefreshToken) error {
	return s.d1.Exec(ctx,
		`INSERT INTO oidc_refresh_token
		   (id, token, client_id, user_id, scopes, audience, amr, auth_time, expiration, created_at)
		 VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10)`,
		rt.ID, rt.Token, rt.ApplicationID, rt.UserID,
		stringsToJSON(rt.Scopes), stringsToJSON(rt.Audience), stringsToJSON(rt.AMR),
		rt.AuthTime.Unix(), rt.Expiration.Unix(), time.Now().Unix())
}

func (s *Storage) refreshTokenByValue(ctx context.Context, token string) (*RefreshToken, error) {
	rows, err := s.d1.Query(ctx,
		`SELECT id, token, client_id, user_id, scopes, audience, amr, auth_time, expiration
		 FROM oidc_refresh_token WHERE token = ?1`, token)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	return &RefreshToken{
		ID:            strVal(row["id"]),
		Token:         strVal(row["token"]),
		ApplicationID: strVal(row["client_id"]),
		UserID:        strVal(row["user_id"]),
		Scopes:        jsonToStrings(row["scopes"]),
		Audience:      jsonToStrings(row["audience"]),
		AMR:           jsonToStrings(row["amr"]),
		AuthTime:      timeVal(row["auth_time"]),
		Expiration:    timeVal(row["expiration"]),
	}, nil
}

func (s *Storage) clientByID(ctx context.Context, clientID string) (*Client, error) {
	rows, err := s.d1.Query(ctx,
		`SELECT id, secret_hash, application_type, auth_method, redirect_uris,
		        post_logout_redirect_uris, grant_types, response_types, scopes,
		        access_token_type, dev_mode, client_name, registration_access_token, created_at
		 FROM oidc_client WHERE id = ?1`, clientID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return clientFromRow(rows[0]), nil
}

func infoFromRequest(req op.TokenRequest) (clientID string, authTime time.Time, amr []string) {
	if authReq, ok := req.(*AuthRequest); ok {
		return authReq.ApplicationID, authReq.AuthTime, authReq.GetAMR()
	}
	if refreshReq, ok := req.(*RefreshTokenRequest); ok {
		return refreshReq.ApplicationID, refreshReq.AuthTime, refreshReq.AMR
	}
	return "", time.Time{}, nil
}

func hasGrantType(grants []oidc.GrantType, want oidc.GrantType) bool {
	for _, g := range grants {
		if g == want {
			return true
		}
	}
	return false
}
