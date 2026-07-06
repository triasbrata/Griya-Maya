// Package r2 implements object storage against Cloudflare R2 through its
// S3-compatible API. Cloudflare Containers do not receive native R2 bindings,
// so we talk S3 with an R2 access-key pair.
package r2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

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

// deleteBatchLimit is the S3 DeleteObjects hard cap (1000 keys per request).
const deleteBatchLimit = 1000

// DeleteObjects removes the given object keys from the bucket, chunking into
// batches of ≤1000 (the S3 DeleteObjects limit). Empty keys are skipped and an
// empty/all-empty input is a no-op. A missing object is treated as success (R2
// omits NoSuchKey from the per-key error list), so cleanup is idempotent.
// Per-key and per-batch failures are aggregated so one bad key doesn't hide the
// rest.
func (s *Store) DeleteObjects(ctx context.Context, keys []string) error {
	pending := make([]string, 0, len(keys))
	for _, k := range keys {
		if strings.TrimSpace(k) != "" {
			pending = append(pending, k)
		}
	}
	if len(pending) == 0 {
		return nil
	}

	var errs []error
	for start := 0; start < len(pending); start += deleteBatchLimit {
		end := start + deleteBatchLimit
		if end > len(pending) {
			end = len(pending)
		}
		objs := make([]s3types.ObjectIdentifier, 0, end-start)
		for _, k := range pending[start:end] {
			objs = append(objs, s3types.ObjectIdentifier{Key: aws.String(k)})
		}
		out, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &s3types.Delete{Objects: objs, Quiet: aws.Bool(true)},
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("r2 delete objects: %w", err))
			continue
		}
		for _, e := range out.Errors {
			key, code, msg := "", "", ""
			if e.Key != nil {
				key = *e.Key
			}
			if e.Code != nil {
				code = *e.Code
			}
			if e.Message != nil {
				msg = *e.Message
			}
			// A key that is already gone is a successful cleanup.
			if code == "NoSuchKey" {
				continue
			}
			errs = append(errs, fmt.Errorf("r2 delete %q: %s (%s)", key, msg, code))
		}
	}
	return errors.Join(errs...)
}

// PresignGet returns a short-lived SigV4 presigned GET URL for key, letting the
// client fetch the object straight from R2 with no container proxy hop. The
// signature lives in the query string (no auth header needed) and self-expires
// after ttl (SigV4 max 7d). Used to gate page bytes behind an IdP-minted URL
// while keeping the bucket fully private.
func (s *Store) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := s3.NewPresignClient(s.client).PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("r2 presign %q: %w", key, err)
	}
	return req.URL, nil
}

// PresignPut mints a short-lived SigV4 presigned PUT URL for key, letting the
// client upload the object straight to R2 with no container proxy hop. The
// signature lives in the query string and self-expires after ttl. When
// contentType is set the client must send a matching Content-Type header on the
// PUT (it is part of the signature); when empty, no content type is bound.
func (s *Store) PresignPut(ctx context.Context, key string, ttl time.Duration, contentType string) (string, error) {
	in := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	req, err := s3.NewPresignClient(s.client).PresignPutObject(ctx, in, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("r2 presign put %q: %w", key, err)
	}
	return req.URL, nil
}

// PublicURL returns a directly-fetchable URL for key when a public/custom R2
// domain is configured, otherwise the empty string (caller proxies instead).
func (s *Store) PublicURL(key string) string {
	if s.publicBaseURL == "" {
		return ""
	}
	return s.publicBaseURL + "/" + strings.TrimLeft(key, "/")
}
