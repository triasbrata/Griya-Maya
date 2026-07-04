package domain

// NovelRegisterRequest associates a text chapter (stored as a `.txt` in R2) with
// a chapter, so the reader endpoint serves that chapter as a single novel page.
//
// Provide either Text (inline; the server writes it to R2) or TextKey (the R2
// key of an already-uploaded `.txt`). Registration rewrites the chapter's pages
// to one PageKindNovel page pointing at that object.
type NovelRegisterRequest struct {
	// ChapterID is the catalog chapter these pages belong to (required).
	ChapterID string `json:"chapterId"`
	// Text is the inline chapter text. When set, the server stores it as a `.txt`
	// object in R2 and uses that key. Ignored when TextKey is set.
	Text string `json:"text,omitempty"`
	// TextKey is the R2 object key of an already-uploaded `.txt` (alternative to
	// Text). One of Text or TextKey is required.
	TextKey string `json:"textKey,omitempty"`
}
