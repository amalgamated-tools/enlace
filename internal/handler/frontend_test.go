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
	index := "<html>spa</html>"
	distFS := fstest.MapFS{
		"index.html":       &fstest.MapFile{Data: []byte(index)},
		"assets/app.js":    &fstest.MapFile{Data: []byte("console.log('ok')")},
		"assets/style.css": &fstest.MapFile{Data: []byte("body{}")},
	}

	handler, err := NewFrontendHandler(distFS)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	tests := []struct {
		name               string
		path               string
		wantBody           string
		expectHTML         bool
		expectContentMatch string
	}{
		{
			name:       "root serves index",
			path:       "/",
			wantBody:   index,
			expectHTML: true,
		},
		{
			name:       "spa route serves index",
			path:       "/dashboard",
			wantBody:   index,
			expectHTML: true,
		},
		{
			name:               "javascript asset served directly",
			path:               "/assets/app.js",
			wantBody:           "console.log('ok')",
			expectContentMatch: "javascript",
		},
		{
			name:               "css asset served directly",
			path:               "/assets/style.css",
			wantBody:           "body{}",
			expectContentMatch: "css",
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

			if tt.expectContentMatch != "" {
				if !strings.Contains(contentType, tt.expectContentMatch) {
					t.Fatalf("expected content-type to contain %q, got %q", tt.expectContentMatch, contentType)
				}
				if string(body) != tt.wantBody {
					t.Fatalf("expected asset content %q, got %q", tt.wantBody, string(body))
				}
			}
		})
	}
}
