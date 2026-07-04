package oidc

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// UserAdminHandler manages the admin_user directory (the accounts that can log
// into the embedded OP). It lives in the oidc package because it shares the user
// table and the argon2id password hashing with the login flow — like DCRHandler,
// it is a management surface wired straight into the router. Responses use the
// uniform domain.APIResponse envelope, matching the rest of the REST layer.
type UserAdminHandler struct {
	storage *Storage
}

// NewUserAdminHandler builds the handler from the OIDC provider's storage.
func NewUserAdminHandler(p *Provider) *UserAdminHandler {
	return &UserAdminHandler{storage: p.storage}
}

// userRecord is the safe (password-free) view of an admin user.
type userRecord struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	EmailVerified bool   `json:"email_verified"`
	CreatedAt     int64  `json:"created_at"`
}

type createUserRequest struct {
	Email         string `json:"email"`
	Name          string `json:"name"`
	Password      string `json:"password"`
	EmailVerified *bool  `json:"email_verified"`
}

type updateUserRequest struct {
	Email         string `json:"email"`
	Name          string `json:"name"`
	Password      string `json:"password"` // optional; blank leaves it unchanged
	EmailVerified *bool  `json:"email_verified"`
}

// List returns all admin users.
//
// @Summary     List admin users
// @Tags        users
// @Produce     json
// @Success     200 {array} oidc.userRecord
// @Failure     401 {object} oidc.userErrorBody
// @Failure     403 {object} oidc.userErrorBody
// @Security    BearerAuth
// @Router      /v1/users [get]
func (h *UserAdminHandler) List(ctx context.Context, c *app.RequestContext) {
	users, err := h.storage.listUsers(ctx)
	if err != nil {
		respondErr(c, consts.StatusInternalServerError, "internal_error", "could not list users")
		return
	}
	respondData(c, consts.StatusOK, users)
}

// Get returns a single admin user by id.
//
// @Summary     Get an admin user
// @Tags        users
// @Produce     json
// @Param       id path string true "User ID"
// @Success     200 {object} oidc.userRecord
// @Failure     404 {object} oidc.userErrorBody
// @Security    BearerAuth
// @Router      /v1/users/{id} [get]
func (h *UserAdminHandler) Get(ctx context.Context, c *app.RequestContext) {
	user, err := h.storage.userByID(ctx, c.Param("id"))
	if err != nil {
		respondErr(c, consts.StatusInternalServerError, "internal_error", "could not load user")
		return
	}
	if user == nil {
		respondErr(c, consts.StatusNotFound, "not_found", "user not found")
		return
	}
	respondData(c, consts.StatusOK, toRecord(user))
}

// Create adds a new admin user.
//
// @Summary     Create an admin user
// @Tags        users
// @Accept      json
// @Produce     json
// @Param       request body oidc.createUserRequest true "New user"
// @Success     201 {object} oidc.userRecord
// @Failure     400 {object} oidc.userErrorBody
// @Failure     409 {object} oidc.userErrorBody
// @Security    BearerAuth
// @Router      /v1/users [post]
func (h *UserAdminHandler) Create(ctx context.Context, c *app.RequestContext) {
	var req createUserRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		respondErr(c, consts.StatusBadRequest, "invalid_request", "malformed JSON body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if !validEmail(req.Email) {
		respondErr(c, consts.StatusBadRequest, "invalid_request", "a valid email is required")
		return
	}
	if len(req.Password) < 8 {
		respondErr(c, consts.StatusBadRequest, "invalid_request", "password must be at least 8 characters")
		return
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

	rec, err := h.storage.createUser(ctx, req.Email, req.Name, req.Password, boolOr(req.EmailVerified, true))
	if err != nil {
		respondErr(c, consts.StatusInternalServerError, "internal_error", "could not create user")
		return
	}
	respondData(c, consts.StatusCreated, rec)
}

// Update edits an existing admin user. A blank password leaves it unchanged.
//
// @Summary     Update an admin user
// @Tags        users
// @Accept      json
// @Produce     json
// @Param       id path string true "User ID"
// @Param       request body oidc.updateUserRequest true "Updated fields"
// @Success     200 {object} oidc.userRecord
// @Failure     400 {object} oidc.userErrorBody
// @Failure     404 {object} oidc.userErrorBody
// @Failure     409 {object} oidc.userErrorBody
// @Security    BearerAuth
// @Router      /v1/users/{id} [put]
func (h *UserAdminHandler) Update(ctx context.Context, c *app.RequestContext) {
	id := c.Param("id")
	current, err := h.storage.userByID(ctx, id)
	if err != nil {
		respondErr(c, consts.StatusInternalServerError, "internal_error", "could not load user")
		return
	}
	if current == nil {
		respondErr(c, consts.StatusNotFound, "not_found", "user not found")
		return
	}

	var req updateUserRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		respondErr(c, consts.StatusBadRequest, "invalid_request", "malformed JSON body")
		return
	}
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		email = current.Email
	}
	if !validEmail(email) {
		respondErr(c, consts.StatusBadRequest, "invalid_request", "a valid email is required")
		return
	}
	if req.Password != "" && len(req.Password) < 8 {
		respondErr(c, consts.StatusBadRequest, "invalid_request", "password must be at least 8 characters")
		return
	}
	// Reject an email collision with a *different* user.
	if other, err := h.storage.userByEmail(ctx, email); err == nil && other != nil && other.ID != id {
		respondErr(c, consts.StatusConflict, "conflict", "a user with that email already exists")
		return
	}

	name := current.Name
	if req.Name != "" {
		name = req.Name
	}
	rec, err := h.storage.updateUser(ctx, id, email, name, boolOr(req.EmailVerified, current.EmailVerified), req.Password)
	if err != nil {
		respondErr(c, consts.StatusInternalServerError, "internal_error", "could not update user")
		return
	}
	respondData(c, consts.StatusOK, rec)
}

// Delete removes an admin user, refusing to delete the last remaining account so
// the panel can't lock everyone out.
//
// @Summary     Delete an admin user
// @Tags        users
// @Produce     json
// @Param       id path string true "User ID"
// @Success     204 "No Content"
// @Failure     404 {object} oidc.userErrorBody
// @Failure     409 {object} oidc.userErrorBody
// @Security    BearerAuth
// @Router      /v1/users/{id} [delete]
func (h *UserAdminHandler) Delete(ctx context.Context, c *app.RequestContext) {
	id := c.Param("id")
	user, err := h.storage.userByID(ctx, id)
	if err != nil {
		respondErr(c, consts.StatusInternalServerError, "internal_error", "could not load user")
		return
	}
	if user == nil {
		respondErr(c, consts.StatusNotFound, "not_found", "user not found")
		return
	}
	n, err := h.storage.userCount(ctx)
	if err != nil {
		respondErr(c, consts.StatusInternalServerError, "internal_error", "could not count users")
		return
	}
	if n <= 1 {
		respondErr(c, consts.StatusConflict, "conflict", "cannot delete the last admin user")
		return
	}
	if err := h.storage.deleteUser(ctx, id); err != nil {
		respondErr(c, consts.StatusInternalServerError, "internal_error", "could not delete user")
		return
	}
	c.Status(consts.StatusNoContent)
}

// --- storage operations (admin_user CRUD) ---

func (s *Storage) listUsers(ctx context.Context) ([]userRecord, error) {
	rows, err := s.d1.Query(ctx,
		`SELECT id, email, name, email_verified, created_at FROM admin_user ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	out := make([]userRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, userRecord{
			ID:            strVal(row["id"]),
			Email:         strVal(row["email"]),
			Name:          strVal(row["name"]),
			EmailVerified: intVal(row["email_verified"]) != 0,
			CreatedAt:     int64(intVal(row["created_at"])),
		})
	}
	return out, nil
}

func (s *Storage) userCount(ctx context.Context) (int, error) {
	rows, err := s.d1.Query(ctx, `SELECT COUNT(*) AS n FROM admin_user`)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return intVal(rows[0]["n"]), nil
}

func (s *Storage) createUser(ctx context.Context, email, name, password string, emailVerified bool) (userRecord, error) {
	hash, err := hashPassword(password)
	if err != nil {
		return userRecord{}, err
	}
	id := uuid.NewString()
	now := time.Now().Unix()
	if err := s.d1.Exec(ctx,
		`INSERT INTO admin_user (id, email, password_hash, name, email_verified, created_at)
		 VALUES (?1, ?2, ?3, ?4, ?5, ?6)`,
		id, email, hash, name, boolToInt(emailVerified), now); err != nil {
		return userRecord{}, err
	}
	return userRecord{ID: id, Email: email, Name: name, EmailVerified: emailVerified, CreatedAt: now}, nil
}

func (s *Storage) updateUser(ctx context.Context, id, email, name string, emailVerified bool, password string) (userRecord, error) {
	if password != "" {
		hash, err := hashPassword(password)
		if err != nil {
			return userRecord{}, err
		}
		if err := s.d1.Exec(ctx,
			`UPDATE admin_user SET email = ?2, name = ?3, email_verified = ?4, password_hash = ?5 WHERE id = ?1`,
			id, email, name, boolToInt(emailVerified), hash); err != nil {
			return userRecord{}, err
		}
	} else if err := s.d1.Exec(ctx,
		`UPDATE admin_user SET email = ?2, name = ?3, email_verified = ?4 WHERE id = ?1`,
		id, email, name, boolToInt(emailVerified)); err != nil {
		return userRecord{}, err
	}
	updated, err := s.userByID(ctx, id)
	if err != nil || updated == nil {
		return userRecord{ID: id, Email: email, Name: name, EmailVerified: emailVerified}, err
	}
	return toRecord(updated), nil
}

func (s *Storage) deleteUser(ctx context.Context, id string) error {
	return s.d1.Exec(ctx, `DELETE FROM admin_user WHERE id = ?1`, id)
}

// --- helpers ---

func toRecord(u *adminUser) userRecord {
	return userRecord{ID: u.ID, Email: u.Email, Name: u.Name, EmailVerified: u.EmailVerified}
}

func validEmail(email string) bool {
	at := strings.IndexByte(email, '@')
	return at > 0 && at < len(email)-1 && !strings.ContainsAny(email, " \t")
}

func boolOr(p *bool, def bool) bool {
	if p != nil {
		return *p
	}
	return def
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// respondData / respondErr emit the uniform response envelope
// (domain.APIResponse) so /v1/users matches every other REST endpoint: success
// carries the typed resource under `data`, errors carry the flat
// `message` + `error_code`.
func respondData[T any](c *app.RequestContext, status int, data T) {
	c.JSON(status, domain.APIResponse[T]{Success: true, Data: data})
}

func respondErr(c *app.RequestContext, status int, code, message string) {
	c.JSON(status, domain.APIResponse[any]{Success: false, ErrorCode: code, Message: message})
}
