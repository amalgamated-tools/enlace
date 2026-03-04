package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestFrontendHandler_ServeHTTP(t *testing.T) {
	distFS := fstest.MapFS{
		"index.html":       &fstest.MapFile{Data: []byte("<html>spa</html>")},
		"assets/app.js":    &fstest.MapFile{Data: []byte("console.log('ok')")},
		"assets/style.css": &fstest.MapFile{Data: []byte("body{}")},
	}

	handler, err := NewFrontendHandler(distFS)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	tests := []struct {
		name            string
		path            string
		wantBody        string
		expectHTML      bool
		expectAssetText string
	}{
		{
			name:       "root serves index",
			path:       "/",
			wantBody:   "<html>spa</html>",
			expectHTML: true,
		},
		{
			name:       "spa route serves index",
			path:       "/dashboard",
			wantBody:   "<html>spa</html>",
			expectHTML: true,
		},
		{
			name:            "static asset served directly",
			path:            "/assets/app.js",
			expectAssetText: "console.log('ok')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			resp := rec.Result()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.StatusCode)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read body: %v", err)
			}

			contentType := resp.Header.Get("Content-Type")

			if tt.expectHTML {
				if ct := contentType; ct != "text/html; charset=utf-8" {
					t.Fatalf("expected html content-type, got %q", ct)
				}
				if string(body) != tt.wantBody {
					t.Fatalf("expected index.html body, got %q", string(body))
				}
				return
			}

			if len(tt.expectAssetText) > 0 {
				if len(tt.wantBody) > 0 && string(body) == tt.wantBody {
					t.Fatalf("expected asset body, got index.html content")
				}
				if !strings.Contains(contentType, "javascript") && !strings.Contains(contentType, "text/plain") {
					t.Fatalf("unexpected content-type for asset: %q", contentType)
				}
				if string(body) != tt.expectAssetText {
					t.Fatalf("expected asset content %q, got %q", tt.expectAssetText, string(body))
				}
			}
		})
	}
}
