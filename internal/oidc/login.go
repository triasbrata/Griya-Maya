package oidc

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// loginUI serves the htmx login + consent screens.
type loginUI struct {
	storage  *Storage
	callback func(context.Context, string) string
}

func newLoginUI(storage *Storage, callback func(context.Context, string) string) *loginUI {
	return &loginUI{storage: storage, callback: callback}
}

// username handles GET (render form) and POST (verify credentials -> consent).
func (l *loginUI) username(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		id := r.URL.Query().Get("authRequestID")
		// A passkey login attaches the subject to the auth request via the JSON
		// endpoints (webauthn_login.go) without rendering anything. When consent is
		// still pending afterwards, the browser navigates here to complete it — so
		// render the consent card directly instead of asking for a password again.
		if id != "" {
			if req, err := l.storage.AuthRequestByID(r.Context(), id); err == nil && req.GetSubject() != "" {
				granted, _ := l.storage.consentScopes(r.Context(), req.GetSubject(), req.GetClientID())
				if missing := missingScopes(req.GetScopes(), granted); len(missing) > 0 {
					clientName := req.GetClientID()
					if c, err := l.storage.clientByID(r.Context(), req.GetClientID()); err == nil && c != nil && c.clientName != "" {
						clientName = c.clientName
					}
					renderPage(w, loginData{ID: id, Consent: true, ClientName: clientName, Scopes: scopeViews(missing)})
					return
				}
			}
		}
		renderPage(w, loginData{ID: id, WebAuthn: l.storage.web != nil})
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "cannot parse form", http.StatusBadRequest)
		return
	}
	id := r.FormValue("id")
	email := r.FormValue("email")
	password := r.FormValue("password")

	uid, err := l.storage.verifyUser(r.Context(), email, password)
	if err != nil {
		// Unverified accounts get the dedicated "under review" intercept page
		// rather than a generic credential error, so a registered-but-pending
		// user knows their sign-in is blocked on owner approval, not a typo.
		if errors.Is(err, errEmailNotVerified) {
			renderCard(w, loginData{ID: id, Email: email, Pending: true})
			return
		}
		renderCard(w, loginData{ID: id, Error: "Invalid email or password", WebAuthn: l.storage.web != nil})
		return
	}
	if err := l.storage.setAuthRequestUser(r.Context(), id, uid); err != nil {
		renderCard(w, loginData{ID: id, Error: "Login session expired, please retry"})
		return
	}

	req, err := l.storage.AuthRequestByID(r.Context(), id)
	if err != nil {
		renderCard(w, loginData{ID: id, Error: "Login session expired, please retry"})
		return
	}

	// Skip the consent screen when the user has already granted every requested
	// scope to this client on a prior sign-in. Only the not-yet-consented scopes
	// (the delta) are shown when consent is still needed (incremental consent).
	granted, _ := l.storage.consentScopes(r.Context(), uid, req.GetClientID())
	missing := missingScopes(req.GetScopes(), granted)
	if len(missing) == 0 {
		if err := l.storage.finishAuthRequest(r.Context(), id); err != nil {
			renderCard(w, loginData{ID: id, Error: "Login session expired, please retry"})
			return
		}
		w.Header().Set("HX-Redirect", l.callback(r.Context(), id))
		w.WriteHeader(http.StatusOK)
		return
	}

	clientName := req.GetClientID()
	if c, err := l.storage.clientByID(r.Context(), req.GetClientID()); err == nil && c != nil && c.clientName != "" {
		clientName = c.clientName
	}
	renderCard(w, loginData{
		ID:         id,
		Consent:    true,
		ClientName: clientName,
		Scopes:     scopeViews(missing),
	})
}

// register handles GET (render the invite-gated signup form) and POST (redeem an
// invite and create an unverified account). It mirrors the JSON Register handler
// (register.go) but renders the login card partials so a browser user can sign
// up from the login screen. The authRequestID is threaded through so, after
// signing up, the user can return to the pending auth flow via "Sign in".
func (l *loginUI) register(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		renderPage(w, loginData{ID: r.URL.Query().Get("authRequestID"), Register: true})
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "cannot parse form", http.StatusBadRequest)
		return
	}
	data := loginData{
		ID:       r.FormValue("id"),
		Register: true,
		Code:     strings.TrimSpace(r.FormValue("code")),
		Email:    strings.TrimSpace(strings.ToLower(r.FormValue("email"))),
		Name:     strings.TrimSpace(r.FormValue("name")),
	}
	password := r.FormValue("password")

	fail := func(msg string) {
		data.Error = msg
		renderCard(w, data)
	}
	switch {
	case !validEmail(data.Email):
		fail("A valid email is required")
		return
	case len(password) < 8:
		fail("Password must be at least 8 characters")
		return
	}

	// The invite code is optional: a valid code creates a verified account that
	// can sign in immediately; without one the account stays unverified pending
	// admin approval (see register.go).
	verified := false
	if data.Code != "" {
		inv, err := l.storage.inviteByCode(r.Context(), data.Code)
		if err != nil {
			fail("Could not check invite, please retry")
			return
		}
		if err := validateInvite(inv, data.Email, time.Now().Unix()); err != nil {
			fail(err.Error())
			return
		}
		verified = true
	}

	existing, err := l.storage.userByEmail(r.Context(), data.Email)
	if err != nil {
		fail("Could not check email, please retry")
		return
	}
	if existing != nil {
		fail("A user with that email already exists")
		return
	}

	rec, err := l.storage.createUser(r.Context(), data.Email, data.Name, password, verified)
	if err != nil {
		fail("Could not create account, please retry")
		return
	}
	if data.Code != "" {
		_ = l.storage.consumeInvite(r.Context(), data.Code, rec.ID)
	}
	renderCard(w, loginData{ID: data.ID, Email: data.Email, Success: true, Verified: verified})
}

// consent finalizes (approve) or aborts (deny) the auth request. On approval it
// redirects the browser (via htmx HX-Redirect) to the OP auth callback.
func (l *loginUI) consent(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "cannot parse form", http.StatusBadRequest)
		return
	}
	id := r.FormValue("id")
	if r.FormValue("action") != "approve" {
		w.Header().Set("HX-Redirect", "/logged-out")
		w.WriteHeader(http.StatusOK)
		return
	}
	// Remember this grant so a later auth request for the same (or a subset of
	// these) scopes skips the consent screen. Derived from the auth request on
	// the server, not the form, so the client can't widen what it recorded.
	if req, err := l.storage.AuthRequestByID(r.Context(), id); err == nil && req.GetSubject() != "" {
		if serr := l.storage.saveConsent(r.Context(), req.GetSubject(), req.GetClientID(), req.GetScopes()); serr != nil {
			slog.Warn("oidc: consent not persisted", "err", serr)
		}
	}
	if err := l.storage.finishAuthRequest(r.Context(), id); err != nil {
		renderCard(w, loginData{ID: id, Consent: true, Error: "Login session expired, please retry"})
		return
	}
	w.Header().Set("HX-Redirect", l.callback(r.Context(), id))
	w.WriteHeader(http.StatusOK)
}

func renderPage(w http.ResponseWriter, data loginData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := loginTemplates.ExecuteTemplate(w, "page", data); err != nil {
		slog.Error("oidc: render login page", "err", err)
	}
}

func renderCard(w http.ResponseWriter, data loginData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := loginTemplates.ExecuteTemplate(w, "card", data); err != nil {
		slog.Error("oidc: render login card", "err", err)
	}
}

// setAuthRequestUser records the verified subject on the pending auth request
// (before consent), extending its KV TTL.
func (s *Storage) setAuthRequestUser(ctx context.Context, id, userID string) error {
	req := &AuthRequest{}
	if err := s.getJSON(ctx, kvAuthReqPrefix+id, req); err != nil {
		return err
	}
	req.UserID = userID
	return s.putJSON(ctx, kvAuthReqPrefix+id, req, s.authReqTTL)
}

// finishAuthRequest marks the auth request authenticated (via password) so the
// OP will issue a code on the callback.
func (s *Storage) finishAuthRequest(ctx context.Context, id string) error {
	return s.finishAuthRequestAMR(ctx, id, "pwd")
}

// finishAuthRequestAMR is finishAuthRequest with an explicit auth method (AMR),
// e.g. "webauthn" for a passkey login.
func (s *Storage) finishAuthRequestAMR(ctx context.Context, id, amr string) error {
	req := &AuthRequest{}
	if err := s.getJSON(ctx, kvAuthReqPrefix+id, req); err != nil {
		return err
	}
	req.IsDone = true
	req.AuthTime = time.Now()
	req.AuthMethod = amr
	return s.putJSON(ctx, kvAuthReqPrefix+id, req, s.authReqTTL)
}
