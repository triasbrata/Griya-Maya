package oidc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/triasbrata/mihon-manga-server/internal/config"
)

// KV key prefixes for the ephemeral WebAuthn ceremony session (challenge etc.).
// Registration is keyed by the authenticated subject; login by the in-flight
// auth request id (the passkey login is discoverable, so no subject is known
// until the assertion is verified).
const (
	kvWebAuthnRegPrefix   = "oidc:webauthn:reg:"
	kvWebAuthnLoginPrefix = "oidc:webauthn:login:"
	webAuthnSessionTTL    = 5 * time.Minute
)

// appleAppSiteAssociation builds the AASA document declaring the webcredentials
// service for the given Apple app IDs (TEAMID.BUNDLEID), enabling native iOS
// passkeys to associate with this domain. Returns nil when no app IDs are set.
func appleAppSiteAssociation(appIDs []string) []byte {
	if len(appIDs) == 0 {
		return nil
	}
	doc := map[string]any{
		"webcredentials": map[string]any{"apps": appIDs},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return nil
	}
	return b
}

// newWebAuthn builds the relying party from config. It returns (nil, nil) when
// WEBAUTHN_RP_ID is empty so an unconfigured deployment (e.g. local dev) simply
// runs with passkeys disabled rather than failing to boot.
func newWebAuthn(cfg config.OIDCConfig) (*webauthn.WebAuthn, error) {
	if cfg.WebAuthnRPID == "" {
		return nil, nil
	}
	return webauthn.New(&webauthn.Config{
		RPID:          cfg.WebAuthnRPID,
		RPDisplayName: cfg.WebAuthnRPDisplayName,
		RPOrigins:     cfg.WebAuthnRPOrigins,
		// Passkeys: require a discoverable (resident) credential so the login
		// ceremony can be usernameless, and prefer user verification (biometric).
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationPreferred,
		},
	})
}

// webauthnUser adapts an admin_user plus its stored credentials to the
// webauthn.User interface. The user handle is the admin_user.id bytes.
type webauthnUser struct {
	user  *adminUser
	creds []webauthn.Credential
}

func (u *webauthnUser) WebAuthnID() []byte { return []byte(u.user.ID) }
func (u *webauthnUser) WebAuthnName() string { return u.user.Email }
func (u *webauthnUser) WebAuthnDisplayName() string {
	if u.user.Name != "" {
		return u.user.Name
	}
	return u.user.Email
}
func (u *webauthnUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

// loadWebAuthnUser assembles the webauthn.User for a subject, loading the admin
// user and all its registered credentials.
func (s *Storage) loadWebAuthnUser(ctx context.Context, userID string) (*webauthnUser, error) {
	u, err := s.userByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, fmt.Errorf("user %q not found", userID)
	}
	creds, err := s.credentialsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &webauthnUser{user: u, creds: creds}, nil
}

// credentialsByUser loads and decodes every stored credential for a user.
func (s *Storage) credentialsByUser(ctx context.Context, userID string) ([]webauthn.Credential, error) {
	rows, err := s.d1.Query(ctx,
		`SELECT data FROM webauthn_credential WHERE user_id = ?1`, userID)
	if err != nil {
		return nil, fmt.Errorf("webauthn creds query: %w", err)
	}
	creds := make([]webauthn.Credential, 0, len(rows))
	for _, row := range rows {
		var c webauthn.Credential
		if err := json.Unmarshal([]byte(strVal(row["data"])), &c); err != nil {
			return nil, fmt.Errorf("webauthn cred decode: %w", err)
		}
		creds = append(creds, c)
	}
	return creds, nil
}

// addCredential persists a newly registered credential for a user.
func (s *Storage) addCredential(ctx context.Context, userID, name string, cred *webauthn.Credential) error {
	data, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("webauthn cred encode: %w", err)
	}
	now := time.Now().Unix()
	return s.d1.Exec(ctx,
		`INSERT INTO webauthn_credential (id, user_id, name, data, created_at, last_used_at)
		 VALUES (?1, ?2, ?3, ?4, ?5, ?5)`,
		credentialKey(cred.ID), userID, name, string(data), now)
}

// touchCredential writes back a credential after a successful login (its sign
// count and other mutable state) and stamps last_used_at.
func (s *Storage) touchCredential(ctx context.Context, cred *webauthn.Credential) error {
	data, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("webauthn cred encode: %w", err)
	}
	return s.d1.Exec(ctx,
		`UPDATE webauthn_credential SET data = ?2, last_used_at = ?3 WHERE id = ?1`,
		credentialKey(cred.ID), string(data), time.Now().Unix())
}

// credentialKey is the base64url encoding of a raw credential ID (the table PK).
func credentialKey(rawID []byte) string {
	return base64.RawURLEncoding.EncodeToString(rawID)
}

// putWebAuthnSession / getWebAuthnSession store the in-flight ceremony session
// (challenge etc.) in KV with a short TTL.
func (s *Storage) putWebAuthnSession(ctx context.Context, key string, sess *webauthn.SessionData) error {
	return s.putJSON(ctx, key, sess, webAuthnSessionTTL)
}

func (s *Storage) getWebAuthnSession(ctx context.Context, key string) (*webauthn.SessionData, error) {
	sess := &webauthn.SessionData{}
	if err := s.getJSON(ctx, key, sess); err != nil {
		return nil, err
	}
	return sess, nil
}
