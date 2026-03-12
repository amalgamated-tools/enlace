package service

import "testing"

func TestSanitizeFilename_Invalid(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		" ",
		".",
		"..",
		"/",
		"\\",
		"bad\x00name.txt",
	}

	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			t.Parallel()
			if _, err := sanitizeFilename(tc); err != ErrInvalidFilename {
				t.Fatalf("expected ErrInvalidFilename for %q, got %v", tc, err)
			}
		})
	}
}

func TestSanitizeFilename_Valid(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		" file.txt ":         "file.txt",
		"subdir/file.txt":    "file.txt",
		"subdir\\file.txt":   "file.txt",
		"..//safe.txt":       "safe.txt",
		"../secret.txt":      "secret.txt",
		"..\\secret.txt":     "secret.txt",
		"dot.name.txt":       "dot.name.txt",
		"normal-file.tar.gz": "normal-file.tar.gz",
	}

	for input, expected := range cases {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			got, err := sanitizeFilename(input)
			if err != nil {
				t.Fatalf("expected no error for %q, got %v", input, err)
			}
			if got != expected {
				t.Fatalf("expected %q, got %q", expected, got)
			}
		})
	}
}
