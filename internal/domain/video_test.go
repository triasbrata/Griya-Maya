package domain

import "testing"

func TestHLSContentType(t *testing.T) {
	cases := map[string]string{
		"index.m3u8":     "application/vnd.apple.mpegurl",
		"seg00001.ts":    "video/mp2t",
		"init.mp4":       "video/mp4",
		"rung0_0.m4s":    "video/mp4",
		"subs.vtt":       "text/vtt",
		"audio.aac":      "audio/aac",
		"README":         "application/octet-stream",
		"weird.unknown":  "application/octet-stream",
		"UPPER.M3U8":     "application/vnd.apple.mpegurl", // extension match is case-insensitive
	}
	for name, want := range cases {
		if got := HLSContentType(name); got != want {
			t.Errorf("HLSContentType(%q) = %q, want %q", name, got, want)
		}
	}
}
