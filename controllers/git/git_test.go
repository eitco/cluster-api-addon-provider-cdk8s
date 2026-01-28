package git

import (
	"os"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-logr/logr"
)

func TestGitURLHelpers(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedIsURL   bool
		expectedURLType authType
	}{
		{
			name:            "HTTPS URL",
			input:           "https://github.com/user/repo",
			expectedIsURL:   true,
			expectedURLType: authTypeHTTP,
		},
		{
			name:            "HTTP URL",
			input:           "http://example.com/repo",
			expectedIsURL:   true,
			expectedURLType: authTypeHTTP,
		},
		{
			name:            "SSH Protocol URL",
			input:           "ssh://git@github.com/user/repo.git",
			expectedIsURL:   true,
			expectedURLType: authTypeSSH,
		},
		{
			name:            "Git SSH URL (git@)",
			input:           "git@github.com:user/repo.git",
			expectedIsURL:   true,
			expectedURLType: authTypeSSH,
		},
		{
			name:            "Absolute path",
			input:           "/tmp/local-repo",
			expectedIsURL:   false,
			expectedURLType: authTypeUnknown,
		},
		{
			name:            "Relative path",
			input:           "./local-repo",
			expectedIsURL:   false,
			expectedURLType: authTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isURLResult := isURL(tt.input)
			if isURLResult != tt.expectedIsURL {
				t.Errorf("isURL(%q) = %v, expected %v", tt.input, isURLResult, tt.expectedIsURL)
			}

			urlTypeResult := getURLType(tt.input)
			if urlTypeResult != tt.expectedURLType {
				t.Errorf("getURLType(%q) = %v, expected %v", tt.input, urlTypeResult, tt.expectedURLType)
			}
		})
	}
}

func TestCheckAccess(t *testing.T) {
	// Setup a local git repo to test against
	tempDir := t.TempDir()
	repo, err := gogit.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	// Add a dummy file and commit to make the repo not empty
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}
	dummyFile := "dummy"
	err = os.WriteFile(tempDir+"/"+dummyFile, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}
	_, err = worktree.Add(dummyFile)
	if err != nil {
		t.Fatalf("failed to add dummy file: %v", err)
	}
	_, err = worktree.Commit("initial commit", &gogit.CommitOptions{})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	g := &Implementer{}
	logger := logr.Discard()

	t.Run("Public accessible local path", func(t *testing.T) {
		accessible, requiresAuth, err := g.CheckAccess(tempDir, nil, logger)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !accessible {
			t.Errorf("expected accessible to be true")
		}
		if requiresAuth {
			t.Errorf("expected requiresAuth to be false for local path")
		}
	})

	t.Run("Non-existent path", func(t *testing.T) {
		accessible, requiresAuth, err := g.CheckAccess("/non/existent/path", nil, logger)
		if err == nil {
			t.Errorf("expected error for non-existent path")
		}
		if accessible {
			t.Errorf("expected accessible to be false")
		}
		_ = requiresAuth
	})
}
