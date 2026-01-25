/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package git

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

// PullRequest represents a pull request from a Git provider.
type PullRequest struct {
	ID         int    `json:"id"`
	Number     int    `json:"number"`
	Branch     string `json:"branch"`
	HeadSHA    string `json:"head_sha"`
	BaseBranch string `json:"base_branch"`
}

// ProviderClient defines the interface for Git provider operations.
type ProviderClient interface {
	ListPullRequests(ctx context.Context, repoURL string, secretRef []byte, logger logr.Logger) ([]PullRequest, error)
}

// GitHubClient implements the ProviderClient interface for GitHub.
type GitHubClient struct {
	HTTPClient *http.Client
}

// ListPullRequests lists open pull requests for a GitHub repository.
func (c *GitHubClient) ListPullRequests(ctx context.Context, repoURL string, secretRef []byte, logger logr.Logger) ([]PullRequest, error) {
	// Parse repoURL to get owner and repo name.
	// Expected formats:
	// https://github.com/owner/repo
	// git@github.com:owner/repo.git
	owner, repo, err := parseGitHubURL(repoURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse GitHub URL")
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=open", owner, repo)
	logger.Info("Querying GitHub API", "url", apiURL, "tokenPresent", len(secretRef) > 0)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if len(secretRef) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", string(secretRef)))
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from GitHub API: %d", resp.StatusCode)
	}

	var ghPRs []struct {
		Number int `json:"number"`
		Head   struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghPRs); err != nil {
		return nil, errors.Wrap(err, "failed to decode response")
	}

	prs := make([]PullRequest, len(ghPRs))
	for i, ghPR := range ghPRs {
		prs[i] = PullRequest{
			Number:     ghPR.Number,
			Branch:     ghPR.Head.Ref,
			HeadSHA:    ghPR.Head.SHA,
			BaseBranch: ghPR.Base.Ref,
		}
	}

	return prs, nil
}

func parseGitHubURL(repoURL string) (string, string, error) {
	repoURL = strings.TrimSuffix(repoURL, ".git")
	if strings.HasPrefix(repoURL, "https://github.com/") {
		parts := strings.Split(strings.TrimPrefix(repoURL, "https://github.com/"), "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	} else if strings.HasPrefix(repoURL, "git@github.com:") {
		parts := strings.Split(strings.TrimPrefix(repoURL, "git@github.com:"), "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}

	return "", "", fmt.Errorf("unsupported or invalid GitHub URL: %s", repoURL)
}
