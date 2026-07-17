package oidc

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/config"
)

func TestAppleAppSiteAssociation_EmptyIsNil(t *testing.T) {
	assert.Nil(t, appleAppSiteAssociation(nil))
	assert.Nil(t, appleAppSiteAssociation([]string{}))
}

func TestAppleAppSiteAssociation_Document(t *testing.T) {
	raw := appleAppSiteAssociation([]string{"ABCDE12345.cloud.brata.griyamedia", "ABCDE12345.other"})
	require.NotNil(t, raw)

	var doc struct {
		WebCredentials struct {
			Apps []string `json:"apps"`
		} `json:"webcredentials"`
	}
	require.NoError(t, json.Unmarshal(raw, &doc))
	assert.Equal(t, []string{"ABCDE12345.cloud.brata.griyamedia", "ABCDE12345.other"}, doc.WebCredentials.Apps)
}

func TestNewWebAuthn_DisabledWhenNoRPID(t *testing.T) {
	w, err := newWebAuthn(config.OIDCConfig{})
	require.NoError(t, err)
	assert.Nil(t, w, "webauthn should be disabled (nil) when WEBAUTHN_RP_ID is empty")
}

func TestNewWebAuthn_BuildsWhenConfigured(t *testing.T) {
	w, err := newWebAuthn(config.OIDCConfig{
		WebAuthnRPID:          "griyamedia.brata.cloud",
		WebAuthnRPDisplayName: "GriyaMedia",
		WebAuthnRPOrigins:     []string{"https://griyamedia.brata.cloud"},
	})
	require.NoError(t, err)
	require.NotNil(t, w)
}

func TestNewWebAuthn_ErrorsOnBadConfig(t *testing.T) {
	// RPID set but no origins → invalid RP config, surfaced as an error (not a panic).
	_, err := newWebAuthn(config.OIDCConfig{WebAuthnRPID: "griyamedia.brata.cloud"})
	assert.Error(t, err)
}

func TestWebauthnUser_Adapter(t *testing.T) {
	u := &webauthnUser{
		user:  &adminUser{ID: "user-123", Email: "a@example.com", Name: "Alex"},
		creds: []webauthn.Credential{{ID: []byte("cred")}},
	}
	assert.Equal(t, []byte("user-123"), u.WebAuthnID())
	assert.Equal(t, "a@example.com", u.WebAuthnName())
	assert.Equal(t, "Alex", u.WebAuthnDisplayName())
	assert.Len(t, u.WebAuthnCredentials(), 1)
}

func TestWebauthnUser_DisplayNameFallsBackToEmail(t *testing.T) {
	u := &webauthnUser{user: &adminUser{Email: "a@example.com"}}
	assert.Equal(t, "a@example.com", u.WebAuthnDisplayName())
}

func TestCredentialKey_IsRawURLBase64(t *testing.T) {
	raw := []byte{0xff, 0xfe, 0xfd, 0x00, 0x10}
	key := credentialKey(raw)
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(raw), key)

	decoded, err := base64.RawURLEncoding.DecodeString(key)
	require.NoError(t, err)
	assert.Equal(t, raw, decoded)
}

func TestAuthRequest_GetAMR(t *testing.T) {
	assert.Nil(t, (&AuthRequest{IsDone: false}).GetAMR(), "not finished → no AMR")
	assert.Equal(t, []string{"pwd"}, (&AuthRequest{IsDone: true}).GetAMR(), "finished without method → pwd")
	assert.Equal(t, []string{"webauthn"},
		(&AuthRequest{IsDone: true, AuthMethod: "webauthn"}).GetAMR())
}
