package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// Self-service registration at POST /v1/register (public). The invite code is
// optional and controls whether the account is usable immediately: redeeming a
// valid single-use invite (minted by admins via POST /v1/invites, users.write)
// creates a verified account that can log in right away; registering without a
// code creates an unverified account that an admin must approve (flip
// email_verified via PUT /v1/users/{id}) before it can log in. Because admin_user
// carries no per-user scope column — access scopes are granted by the OP from
// what the client requests during login — verifyUser rejects unverified users,
// so the unverified path grants no access until approved. This file lives in the
// oidc package alongside UserAdminHandler because it shares admin_user and
// argon2id hashing.

// errEmailNotVerified is returned by verifyUser when an otherwise-valid account
// has not been verified yet, so the login UI can show a distinct message.
var errEmailNotVerified = errors.New("email not verified")

// inviteRow is the raw persisted invite.
type inviteRow struct {
	Code      string
	Email     string
	Note      string
	ExpiresAt int64
	UsedAt    int64
	UsedBy    string
	CreatedAt int64
}

// inviteRecord is the JSON view returned to admins.
type inviteRecord struct {
	Code      string `json:"code"`
	Email     string `json:"email,omitempty"`
	Note      string `json:"note,omitempty"`
	ExpiresAt int64  `json:"expires_at"`
	UsedAt    int64  `json:"used_at"`
	UsedBy    string `json:"used_by,omitempty"`
	CreatedAt int64  `json:"created_at"`
}

func (r inviteRow) toRecord() inviteRecord {
	return inviteRecord{
		Code:      r.Code,
		Email:     r.Email,
		Note:      r.Note,
		ExpiresAt: r.ExpiresAt,
		UsedAt:    r.UsedAt,
		UsedBy:    r.UsedBy,
		CreatedAt: r.CreatedAt,
	}
}

type createInviteRequest struct {
	Email     string `json:"email"`      // optional; blank = any email may redeem
	Note      string `json:"note"`       // optional admin label
	ExpiresAt int64  `json:"expires_at"` // optional unix seconds; 0 = never
}

type registerRequest struct {
	Code     string `json:"code"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

// validateInvite checks a loaded invite against the redeeming email at time now.
// It is pure (no I/O) so the redemption rules are unit-testable. A nil invite
// means the code did not exist. Returns a redemption-facing error, or nil when
// the invite may be consumed for email.
func validateInvite(inv *inviteRow, email string, now int64) error {
	if inv == nil {
		return errors.New("invalid invite code")
	}
	if inv.UsedAt != 0 {
		return errors.New("invite code has already been used")
	}
	if inv.ExpiresAt != 0 && now > inv.ExpiresAt {
		return errors.New("invite code has expired")
	}
	if inv.Email != "" && !strings.EqualFold(inv.Email, email) {
		return errors.New("this invite is for a different email address")
	}
	return nil
}

// Register redeems an invite and creates an unverified account.
//
// @Summary     Register a new account (invite optional)
// @Description Public self-service signup. The invite code is optional: with a valid single-use code the account is created verified (email_verified=true) and can sign in immediately; without a code it is created unverified and cannot log in until an admin verifies it. A supplied-but-invalid code is rejected.
// @Tags        users
// @Accept      json
// @Produce     json
// @Param       request body oidc.registerRequest true "Registration"
// @Success     201 {object} oidc.userRecord
// @Failure     400 {object} oidc.userErrorBody
// @Failure     403 {object} oidc.userErrorBody
// @Failure     409 {object} oidc.userErrorBody
// @Router      /v1/register [post]
func (h *UserAdminHandler) Register(ctx context.Context, c *app.RequestContext) {
	var req registerRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		respondErr(c, consts.StatusBadRequest, "invalid_request", "malformed JSON body")
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if !validEmail(req.Email) {
		respondErr(c, consts.StatusBadRequest, "invalid_request", "a valid email is required")
		return
	}
	if len(req.Password) < 8 {
		respondErr(c, consts.StatusBadRequest, "invalid_request", "password must be at least 8 characters")
		return
	}

	// The invite code is optional. With a valid code the account is created
	// already verified and can sign in immediately; without one it is created
	// unverified and an admin must approve it (verify email_verified) before it
	// can log in and be granted any scope. A supplied-but-invalid code is an
	// error rather than a silent downgrade to unverified.
	verified := false
	if req.Code != "" {
		inv, err := h.storage.inviteByCode(ctx, req.Code)
		if err != nil {
			respondErr(c, consts.StatusInternalServerError, "internal_error", "could not check invite")
			return
		}
		if err := validateInvite(inv, req.Email, time.Now().Unix()); err != nil {
			respondErr(c, consts.StatusForbidden, "invite_invalid", err.Error())
			return
		}
		verified = true
	}

	existing, err := h.storage.userByEmail(ctx, req.Email)
	if err != nil {
		respondErr(c, consts.StatusInternalServerError, "internal_error", "could not check email")
		return
	}
	if existing != nil {
		respondErr(c, consts.StatusConflict, "conflict", "a user with that email already exists")
		return
	}

	rec, err := h.storage.createUser(ctx, req.Email, req.Name, req.Password, verified)
	if err != nil {
		respondErr(c, consts.StatusInternalServerError, "internal_error", "could not create account")
		return
	}
	// Consume the invite only when one was redeemed. Best-effort: the account
	// already exists, so a failed mark only risks the (email-bound, or single-use)
	// code lingering — surfaced on next redeem by the duplicate-email 409.
	if req.Code != "" {
		_ = h.storage.consumeInvite(ctx, req.Code, rec.ID)
	}
	respondData(c, consts.StatusCreated, rec)
}

// CreateInvite mints a new invite code.
//
// @Summary     Create a registration invite
// @Tags        users
// @Accept      json
// @Produce     json
// @Param       request body oidc.createInviteRequest true "Invite"
// @Success     201 {object} oidc.inviteRecord
// @Failure     400 {object} oidc.userErrorBody
// @Failure     401 {object} oidc.userErrorBody
// @Failure     403 {object} oidc.userErrorBody
// @Security    BearerAuth
// @Router      /v1/invites [post]
func (h *UserAdminHandler) CreateInvite(ctx context.Context, c *app.RequestContext) {
	var req createInviteRequest
	if len(c.Request.Body()) > 0 {
		if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
			respondErr(c, consts.StatusBadRequest, "invalid_request", "malformed JSON body")
			return
		}
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email != "" && !validEmail(req.Email) {
		respondErr(c, consts.StatusBadRequest, "invalid_request", "email must be valid when provided")
		return
	}
	rec, err := h.storage.createInvite(ctx, req.Email, req.Note, req.ExpiresAt)
	if err != nil {
		respondErr(c, consts.StatusInternalServerError, "internal_error", "could not create invite")
		return
	}
	respondData(c, consts.StatusCreated, rec)
}

// ListInvites returns all invites (newest first).
//
// @Summary     List registration invites
// @Tags        users
// @Produce     json
// @Success     200 {array} oidc.inviteRecord
// @Failure     401 {object} oidc.userErrorBody
// @Failure     403 {object} oidc.userErrorBody
// @Security    BearerAuth
// @Router      /v1/invites [get]
func (h *UserAdminHandler) ListInvites(ctx context.Context, c *app.RequestContext) {
	invites, err := h.storage.listInvites(ctx)
	if err != nil {
		respondErr(c, consts.StatusInternalServerError, "internal_error", "could not list invites")
		return
	}
	respondData(c, consts.StatusOK, invites)
}

// DeleteInvite revokes an invite by its code.
//
// @Summary     Revoke a registration invite
// @Tags        users
// @Produce     json
// @Param       code path string true "Invite code"
// @Success     204 "No Content"
// @Failure     401 {object} oidc.userErrorBody
// @Failure     403 {object} oidc.userErrorBody
// @Failure     404 {object} oidc.userErrorBody
// @Security    BearerAuth
// @Router      /v1/invites/{code} [delete]
func (h *UserAdminHandler) DeleteInvite(ctx context.Context, c *app.RequestContext) {
	code := c.Param("code")
	inv, err := h.storage.inviteByCode(ctx, code)
	if err != nil {
		respondErr(c, consts.StatusInternalServerError, "internal_error", "could not load invite")
		return
	}
	if inv == nil {
		respondErr(c, consts.StatusNotFound, "not_found", "invite not found")
		return
	}
	if err := h.storage.deleteInvite(ctx, code); err != nil {
		respondErr(c, consts.StatusInternalServerError, "internal_error", "could not delete invite")
		return
	}
	c.Status(consts.StatusNoContent)
}

// --- storage operations (user_invite CRUD) ---

func inviteFromRow(row map[string]any) *inviteRow {
	return &inviteRow{
		Code:      strVal(row["code"]),
		Email:     strVal(row["email"]),
		Note:      strVal(row["note"]),
		ExpiresAt: int64(intVal(row["expires_at"])),
		UsedAt:    int64(intVal(row["used_at"])),
		UsedBy:    strVal(row["used_by"]),
		CreatedAt: int64(intVal(row["created_at"])),
	}
}

func (s *Storage) createInvite(ctx context.Context, email, note string, expiresAt int64) (inviteRecord, error) {
	row := inviteRow{
		Code:      randToken(24),
		Email:     email,
		Note:      note,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().Unix(),
	}
	if err := s.d1.Exec(ctx,
		`INSERT INTO user_invite (code, email, note, expires_at, used_at, used_by, created_at)
		 VALUES (?1, ?2, ?3, ?4, 0, '', ?5)`,
		row.Code, row.Email, row.Note, row.ExpiresAt, row.CreatedAt); err != nil {
		return inviteRecord{}, err
	}
	return row.toRecord(), nil
}

func (s *Storage) listInvites(ctx context.Context) ([]inviteRecord, error) {
	rows, err := s.d1.Query(ctx,
		`SELECT code, email, note, expires_at, used_at, used_by, created_at
		 FROM user_invite ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	out := make([]inviteRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, inviteFromRow(row).toRecord())
	}
	return out, nil
}

func (s *Storage) inviteByCode(ctx context.Context, code string) (*inviteRow, error) {
	rows, err := s.d1.Query(ctx,
		`SELECT code, email, note, expires_at, used_at, used_by, created_at
		 FROM user_invite WHERE code = ?1`, code)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return inviteFromRow(rows[0]), nil
}

// consumeInvite marks an invite used. The WHERE used_at = 0 guard keeps a
// concurrent double-redeem from clobbering the first consumer's record.
func (s *Storage) consumeInvite(ctx context.Context, code, usedBy string) error {
	return s.d1.Exec(ctx,
		`UPDATE user_invite SET used_at = ?2, used_by = ?3 WHERE code = ?1 AND used_at = 0`,
		code, time.Now().Unix(), usedBy)
}

func (s *Storage) deleteInvite(ctx context.Context, code string) error {
	return s.d1.Exec(ctx, `DELETE FROM user_invite WHERE code = ?1`, code)
}
