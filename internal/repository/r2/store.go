// Package r2 implements object storage against Cloudflare R2 through its
// S3-compatible API. Cloudflare Containers do not receive native R2 bindings,
// so we talk S3 with an R2 access-key pair.
package r2

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/triasbrata/mihon-manga-server/internal/config"
)

// Store is a thin R2 wrapper scoped to a single bucket.
type Store struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
}

// New builds an R2 store from config. It never dials until first use.
func New(cfg config.R2Config) *Store {
	client := s3.New(s3.Options{
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		BaseEndpoint: aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)),
		// R2 requires path-style addressing.
		UsePathStyle: true,
	})
	return &Store{
		client:        client,
		bucket:        cfg.Bucket,
		publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
	}
}

// Get downloads an object fully into memory. Callers should bound object size
// upstream; archives are streamed to a temp file by the convert layer.
func (s *Store) Get(ctx context.Context, key string) ([]byte, string, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("r2 get %q: %w", key, err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, "", fmt.Errorf("r2 read %q: %w", key, err)
	}
	ct := ""
	if out.ContentType != nil {
		ct = *out.ContentType
	}
	return data, ct, nil
}

// Put uploads bytes under key with the given content type.
func (s *Store) Put(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("r2 put %q: %w", key, err)
	}
	return nil
}

// PublicURL returns a directly-fetchable URL for key when a public/custom R2
// domain is configured, otherwise the empty string (caller proxies instead).
func (s *Store) PublicURL(key string) string {
	if s.publicBaseURL == "" {
		return ""
	}
	return s.publicBaseURL + "/" + strings.TrimLeft(key, "/")
}
