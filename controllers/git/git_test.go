package git

import (
	"net/url"
	"testing"
)

/*
var (
	validRepoUrl   = "https://github.com/PatrickLaabs/cdk8s-sample-deployment"
	invalidRepoUrl = "https://github.com/PatrickLaabs/invalid-repo"
	branch         = "main"
)
*/

func TestIsUrl(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expected       bool
		expectedScheme string
	}{
		// Valid URLs (should return true)
		{
			name:           "https URL",
			input:          "https://github.com/user/repo",
			expected:       true,
			expectedScheme: "https",
		},
		{
			name:           "http URL",
			input:          "http://example.com/repo",
			expected:       true,
			expectedScheme: "http",
		},
		{
			name:           "git URL",
			input:          "git://github.com/user/repo.git",
			expected:       true,
			expectedScheme: "git",
		},
		{
			name:           "ssh URL",
			input:          "ssh://git@github.com/user/repo.git",
			expected:       true,
			expectedScheme: "ssh",
		},
		{
			name:           "git+ssh URL",
			input:          "git+ssh://git@github.com/user/repo.git",
			expected:       true,
			expectedScheme: "git+ssh",
		},

		// Directory paths (should return false)
		{
			name:           "absolute path",
			input:          "/tmp/local-repo",
			expected:       false,
			expectedScheme: "", // No scheme for paths
		},
		{
			name:           "relative path with dot",
			input:          "./local-repo",
			expected:       false,
			expectedScheme: "",
		},
		{
			name:           "relative path with double dot",
			input:          "../local-repo",
			expected:       false,
			expectedScheme: "",
		},
		{
			name:           "simple directory name",
			input:          "local-repo",
			expected:       false,
			expectedScheme: "",
		},
		{
			name:           "nested path",
			input:          "path/to/local-repo",
			expected:       false,
			expectedScheme: "",
		},
		{
			name:           "temp directory pattern",
			input:          "/tmp/cdk8s-git-clone-123",
			expected:       false,
			expectedScheme: "",
		},

		// Edge cases
		{
			name:           "empty string",
			input:          "",
			expected:       false,
			expectedScheme: "",
		},
		{
			name:           "just scheme",
			input:          "https://",
			expected:       true,
			expectedScheme: "https",
		},
		{
			name:           "malformed URL",
			input:          "not-a-url",
			expected:       false,
			expectedScheme: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUrl(tt.input)
			if result != tt.expected {
				t.Errorf("isUrl(%q) = %v, expected %v", tt.input, result, tt.expected)
			}

			// Additional test: verify the actual scheme parsing
			if tt.expected || tt.expectedScheme == "" {
				parsedUrl, err := url.ParseRequestURI(tt.input)
				if err != nil && tt.expected {
					t.Errorf("Expected %q to parse successfully, but got error: %v", tt.input, err)
				} else if err == nil {
					if parsedUrl.Scheme != tt.expectedScheme {
						t.Errorf("For input %q, expected scheme %q, but got %q", tt.input, tt.expectedScheme, parsedUrl.Scheme)
					}
				}
			}
		})
	}
}

func TestEmptyChecker(t *testing.T) {
	tests := []struct {
		name      string
		repo      string
		directory string
		expected  bool
	}{
		// Both empty cases
		{
			name:      "both repo and directory empty",
			repo:      "",
			directory: "",
			expected:  true,
		},
		// Single empty cases
		{
			name:      "repo empty, directory not empty",
			repo:      "",
			directory: "/tmp/some-dir",
			expected:  true,
		},
		{
			name:      "repo not empty, directory empty",
			repo:      "https://github.com/user/repo",
			directory: "",
			expected:  true,
		},
		// Both non-empty cases
		{
			name:      "both repo and directory not empty",
			repo:      "https://github.com/user/repo",
			directory: "/tmp/some-dir",
			expected:  false,
		},
		{
			name:      "both repo and directory with local paths",
			repo:      "./local-repo",
			directory: "./target-dir",
			expected:  false,
		},
		// Edge cases with whitespace
		{
			name:      "repo with spaces, directory empty",
			repo:      "   ",
			directory: "",
			expected:  true,
		},
		{
			name:      "repo empty, directory with spaces",
			repo:      "",
			directory: "   ",
			expected:  true,
		},
		{
			name:      "both with spaces (non-empty)",
			repo:      "   ",
			directory: "   ",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := empty(tt.repo, tt.directory)
			if result != tt.expected {
				t.Errorf("emptyChecker(%q, %q) = %v, expected %v", tt.repo, tt.directory, result, tt.expected)
			}
		})
	}
}

// Helper functions.
/*
func setupTestRepo(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()

	t.Cleanup(func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			t.Errorf("Cleaning up temp dir failed: %v", err)
		}
	})

	repo, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	fileName := tempDir + "/cdk8s-sample-deployment.yaml"
	if err := os.WriteFile(fileName, []byte("yaml-content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}
	_, err = w.Add("cdk8s-sample-deployment.yaml")
	if err != nil {
		t.Fatalf("failed to add cdk8s-sample-deployment.yaml: %v", err)
	}

	_, err = w.Commit("cdk8s-sample-deployment.yaml", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Tester",
			Email: "tester@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	return tempDir
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}

				return false
			}())))
}
*/
