package oidc

import (
	"net/http"
	"strings"

	"golang.org/x/text/language"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/triasbrata/mihon-manga-server/internal/config"
)

// Provider bundles the zitadel OpenID Provider, its HTTP handler (login +
// consent + all OIDC endpoints) and the resource-server access-token verifier.
type Provider struct {
	op            *op.Provider
	storage       *Storage
	handler       http.Handler
	requiredScope string
}

// NewProvider constructs the OpenID Provider over the given storage.
func NewProvider(storage *Storage, cfg config.OIDCConfig) (*Provider, error) {
	var key [32]byte
	copy(key[:], []byte(cfg.CryptoKey))

	supportedScopes := []string{
		oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail,
		oidc.ScopeOfflineAccess, ScopeMangaWrite, ScopeMangaRead,
		ScopeConnectionsWrite,
	}
	for _, k := range TaxonomyWriteKinds {
		supportedScopes = append(supportedScopes, ScopeTaxonomyWrite(k))
	}

	opConfig := &op.Config{
		CryptoKey:                key,
		DefaultLogoutRedirectURI: "/logged-out",
		CodeMethodS256:           true,
		AuthMethodPost:           true,
		GrantTypeRefreshToken:    true,
		SupportedUILocales:       []language.Tag{language.English},
		SupportedScopes:          supportedScopes,
	}

	// Disable the OP's built-in CORS layer. zitadel's default echoes any
	// request Origin into Access-Control-Allow-Origin, which would duplicate the
	// header the Hertz server already sets (router.go), and browsers reject a
	// response carrying two Access-Control-Allow-Origin values. CORS is owned
	// solely by the Hertz server.
	opts := []op.Option{op.WithCORSOptions(nil)}
	if strings.HasPrefix(cfg.Issuer, "http://") {
		// Allow the http issuer used in local dev; production uses https.
		opts = append(opts, op.WithAllowInsecure())
	}

	provider, err := op.NewOpenIDProvider(cfg.Issuer, opConfig, storage, opts...)
	if err != nil {
		return nil, err
	}

	interceptor := op.NewIssuerInterceptor(provider.IssuerFromRequest)
	login := newLoginUI(storage, op.AuthCallbackURL(provider))

	mux := http.NewServeMux()
	mux.HandleFunc("/login/username", login.username)
	mux.Handle("/login/consent", interceptor.HandlerFunc(login.consent))
	mux.HandleFunc("/logged-out", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><p>Signed out.</p>"))
	})
	// Everything else (discovery, authorize, token, userinfo, keys, ...) is
	// handled by the OP itself.
	mux.Handle("/", provider.HttpHandler())

	return &Provider{
		op:            provider,
		storage:       storage,
		handler:       mux,
		requiredScope: cfg.RequiredScope,
	}, nil
}

// Handler is the composite net/http handler serving login + all OIDC endpoints.
func (p *Provider) Handler() http.Handler { return p.handler }

// Storage exposes the backing storage (used to build the DCR handler).
func (p *Provider) Storage() *Storage { return p.storage }
