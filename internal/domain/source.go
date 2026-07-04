package domain

import "time"

// Source is a content source in the Mihon SourceRuntime contract. The catalog is
// partitioned by source (media.source_id) and the reader browses per source.
// This server is typically single-source ("griyamedia"), but the model supports
// many and admins manage them.
type Source struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Lang      string    `json:"lang"`
	IconURL   string    `json:"iconUrl,omitempty"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

// SourceWriteRequest is the create/update payload for a source. ID is the stable
// slug the catalog references (required on create, immutable on update); Lang
// defaults to "en" and Enabled to true when omitted.
type SourceWriteRequest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Lang    string `json:"lang,omitempty"`
	IconURL string `json:"iconUrl,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}
