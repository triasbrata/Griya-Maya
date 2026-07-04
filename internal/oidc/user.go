package oidc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// adminUser is a row of admin_user.
type adminUser struct {
	ID            string
	Email         string
	PasswordHash  string
	Name          string
	EmailVerified bool
}

func adminUserFromRow(row map[string]any) *adminUser {
	return &adminUser{
		ID:            strVal(row["id"]),
		Email:         strVal(row["email"]),
		PasswordHash:  strVal(row["password_hash"]),
		Name:          strVal(row["name"]),
		EmailVerified: intVal(row["email_verified"]) != 0,
	}
}

// userByEmail loads an admin user by email, or nil if not found.
func (s *Storage) userByEmail(ctx context.Context, email string) (*adminUser, error) {
	rows, err := s.d1.Query(ctx,
		`SELECT id, email, password_hash, name, email_verified FROM admin_user WHERE email = ?1`, email)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return adminUserFromRow(rows[0]), nil
}

// userByID loads an admin user by subject id, or nil if not found.
func (s *Storage) userByID(ctx context.Context, id string) (*adminUser, error) {
	rows, err := s.d1.Query(ctx,
		`SELECT id, email, password_hash, name, email_verified FROM admin_user WHERE id = ?1`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return adminUserFromRow(rows[0]), nil
}

// verifyUser checks the email/password against admin_user and returns the id.
func (s *Storage) verifyUser(ctx context.Context, email, password string) (string, error) {
	user, err := s.userByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if user == nil || !verifyPassword(password, user.PasswordHash) {
		return "", errors.New("invalid email or password")
	}
	return user.ID, nil
}

// seedAdmin inserts the configured admin user on first boot if the admin_user
// table is empty and a password was provided. It is best-effort: failures are
// logged (D1 may be unconfigured during local dev) but do not abort startup.
func (s *Storage) seedAdmin(ctx context.Context) {
	if s.adminEmail == "" || s.adminPassword == "" {
		return
	}
	rows, err := s.d1.Query(ctx, `SELECT COUNT(*) AS n FROM admin_user`)
	if err != nil {
		slog.Warn("oidc: admin seed skipped (admin_user count failed)", "err", err)
		return
	}
	if len(rows) > 0 && intVal(rows[0]["n"]) > 0 {
		return
	}
	hash, err := hashPassword(s.adminPassword)
	if err != nil {
		slog.Warn("oidc: admin seed failed (hash)", "err", err)
		return
	}
	err = s.d1.Exec(ctx,
		`INSERT INTO admin_user (id, email, password_hash, name, email_verified, created_at)
		 VALUES (?1, ?2, ?3, ?4, 1, ?5)`,
		uuid.NewString(), s.adminEmail, hash, "Administrator", time.Now().Unix())
	if err != nil {
		slog.Warn("oidc: admin seed failed (insert)", "err", err)
		return
	}
	slog.Info("oidc: seeded admin user", "email", s.adminEmail)
}
