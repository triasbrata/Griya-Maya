package oidc

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-webauthn/webauthn/webauthn"
)

// webauthnLoginBegin starts a usernameless (discoverable) passkey login for an
// in-flight auth request. POST /login/webauthn/begin?authRequestID=<id>. It
// returns the CredentialAssertion options JSON for navigator.credentials.get
// (browser) or ASAuthorizationController (native iOS), and stashes the ceremony
// session in KV keyed by the auth request id.
func (l *loginUI) webauthnLoginBegin(w http.ResponseWriter, r *http.Request) {
	if l.storage.web == nil {
		writeHTTPError(w, http.StatusServiceUnavailable, "passkey login is not enabled")
		return
	}
	id := r.URL.Query().Get("authRequestID")
	if id == "" {
		writeHTTPError(w, http.StatusBadRequest, "missing authRequestID")
		return
	}
	if _, err := l.storage.AuthRequestByID(r.Context(), id); err != nil {
		writeHTTPError(w, http.StatusBadRequest, "unknown auth request")
		return
	}

	assertion, session, err := l.storage.web.BeginDiscoverableLogin()
	if err != nil {
		slog.Error("oidc: webauthn login begin", "err", err)
		writeHTTPError(w, http.StatusInternalServerError, "could not start passkey login")
		return
	}
	if err := l.storage.putWebAuthnSession(r.Context(), kvWebAuthnLoginPrefix+id, session); err != nil {
		writeHTTPError(w, http.StatusInternalServerError, "could not start passkey login")
		return
	}
	writeHTTPJSON(w, http.StatusOK, assertion)
}

// webauthnLoginFinish verifies the passkey assertion, attaches the resolved
// subject to the auth request, and returns the next destination as JSON:
//   - {"redirect": "<op callback>"} when no further consent is needed;
//   - {"consentRequired": true, "redirect": "/login/username?authRequestID=<id>"}
//     when the user must still approve scopes (the login page renders the consent
//     card because the subject is now attached — see username()).
//
// POST /login/webauthn/finish?authRequestID=<id> with the credential JSON body.
func (l *loginUI) webauthnLoginFinish(w http.ResponseWriter, r *http.Request) {
	if l.storage.web == nil {
		writeHTTPError(w, http.StatusServiceUnavailable, "passkey login is not enabled")
		return
	}
	id := r.URL.Query().Get("authRequestID")
	if id == "" {
		writeHTTPError(w, http.StatusBadRequest, "missing authRequestID")
		return
	}
	session, err := l.storage.getWebAuthnSession(r.Context(), kvWebAuthnLoginPrefix+id)
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, "login session expired, please retry")
		return
	}

	// The discoverable handler resolves the asserting user from the authenticator's
	// user handle (the admin_user.id we set at registration).
	handler := func(_, userHandle []byte) (webauthn.User, error) {
		return l.storage.loadWebAuthnUser(r.Context(), string(userHandle))
	}
	user, cred, err := l.storage.web.FinishPasskeyLogin(handler, *session, r)
	if err != nil {
		slog.Warn("oidc: webauthn login finish", "err", err)
		writeHTTPError(w, http.StatusUnauthorized, "passkey verification failed")
		return
	}
	userID := string(user.WebAuthnID())

	// Persist the credential's updated sign count / usage.
	if err := l.storage.touchCredential(r.Context(), cred); err != nil {
		slog.Warn("oidc: webauthn credential not updated", "err", err)
	}
	if err := l.storage.setAuthRequestUser(r.Context(), id, userID); err != nil {
		writeHTTPError(w, http.StatusBadRequest, "login session expired, please retry")
		return
	}

	req, err := l.storage.AuthRequestByID(r.Context(), id)
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, "login session expired, please retry")
		return
	}
	granted, _ := l.storage.consentScopes(r.Context(), userID, req.GetClientID())
	if missing := missingScopes(req.GetScopes(), granted); len(missing) > 0 {
		writeHTTPJSON(w, http.StatusOK, map[string]any{
			"consentRequired": true,
			"redirect":        "/login/username?authRequestID=" + id,
		})
		return
	}
	if err := l.storage.finishAuthRequestAMR(r.Context(), id, "webauthn"); err != nil {
		writeHTTPError(w, http.StatusBadRequest, "login session expired, please retry")
		return
	}
	writeHTTPJSON(w, http.StatusOK, map[string]any{"redirect": l.callback(r.Context(), id)})
}

func writeHTTPJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("oidc: write json", "err", err)
	}
}

func writeHTTPError(w http.ResponseWriter, status int, msg string) {
	writeHTTPJSON(w, status, map[string]string{"error": msg})
}
