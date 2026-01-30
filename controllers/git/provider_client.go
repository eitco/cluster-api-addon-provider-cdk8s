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
)

// Provider represents a Git provider type.
type Provider string

const (
	ProviderGitHub    Provider = "github"
	ProviderGitLab    Provider = "gitlab"
	ProviderBitbucket Provider = "bitbucket"
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
	ListPullRequests(ctx context.Context, repoURL string, secretRef []byte) (prs []PullRequest, err error)
}

// NewProviderClient returns the appropriate ProviderClient for the given repoURL.
func NewProviderClient(repoURL string, httpClient *http.Client) (client ProviderClient, err error) {
	provider := DetectProvider(repoURL)

	host := ""
	allowNested := false
	switch provider {
	case ProviderGitHub:
		host = "github.com"
	case ProviderGitLab:
		host = "gitlab.com"
		allowNested = true
	case ProviderBitbucket:
		host = "bitbucket.org"
	}

	return &Client{
		httpClientContainer: httpClientContainer{httpClient: httpClient},
		provider:            provider,
		host:                host,
		allowNested:         allowNested,
	}, nil
}

// httpClientContainer provides common HTTP client access.
type httpClientContainer struct {
	httpClient *http.Client
}

func (h *httpClientContainer) getHTTPClient() *http.Client {
	if h.httpClient == nil {
		return http.DefaultClient
	}

	return h.httpClient
}

func (h *httpClientContainer) doJSONRequest(ctx context.Context, apiURL string, headers map[string]string, target any) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		fmt.Println("replace me")

		return err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := h.getHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)	
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// DetectProvider determines the Git provider based on the repository URL.
func DetectProvider(repoURL string) (provider Provider) {
	repoURL = strings.ToLower(repoURL)
	if strings.Contains(repoURL, "github.com") {
		return ProviderGitHub
	}
	if strings.Contains(repoURL, "gitlab.com") {
		return ProviderGitLab
	}
	if strings.Contains(repoURL, "bitbucket.org") {
		return ProviderBitbucket
	}

	return provider
}

// Client implements the ProviderClient interface for various Git providers.
type Client struct {
	httpClientContainer
	provider    Provider
	host        string
	allowNested bool
}

// ListPullRequests lists open pull requests for the repository.
func (c *Client) ListPullRequests(ctx context.Context, repoURL string, secretRef []byte) (prs []PullRequest, err error) {
	owner, repo, err := parseRepoURL(repoURL, c.host, c.allowNested)
	if err != nil {
		return nil, err
	}

	var apiURL string
	headers := make(map[string]string)

	switch c.provider {
	case ProviderGitHub:
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=open", owner, repo)
		headers["Accept"] = "application/vnd.github.v3+json"
		if len(secretRef) > 0 {
			headers["Authorization"] = fmt.Sprintf("token %s", string(secretRef))
		}

		return c.fetchGitHubPRs(ctx, apiURL, headers)

	case ProviderGitLab:
		projectID := urlPathEscape(fmt.Sprintf("%s/%s", owner, repo))
		apiURL = fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/merge_requests?state=opened", projectID)
		if len(secretRef) > 0 {
			headers["Private-Token"] = string(secretRef)
		}

		return c.fetchGitLabMRs(ctx, apiURL, headers)

	case ProviderBitbucket:
		apiURL = fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s/%s/pullrequests?state=OPEN", owner, repo)
		if len(secretRef) > 0 {
			headers["Authorization"] = fmt.Sprintf("Bearer %s", string(secretRef))
		}

		return c.fetchBitbucketPRs(ctx, apiURL, headers)

	default:
		return nil, fmt.Errorf("unsupported Git provider: %s", c.provider)
	}
}

func (c *Client) fetchGitHubPRs(ctx context.Context, apiURL string, headers map[string]string) (prs []PullRequest, err error) {
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

	if err := c.doJSONRequest(ctx, apiURL, headers, &ghPRs); err != nil {
		return nil, fmt.Errorf("github api request failed: %w", err)
	}

	prs = make([]PullRequest, len(ghPRs))
	for i, ghPR := range ghPRs {
		prs[i] = PullRequest{
			Number:     ghPR.Number,
			Branch:     ghPR.Head.Ref,
			HeadSHA:    ghPR.Head.SHA,
			BaseBranch: ghPR.Base.Ref,
		}
	}

	return prs, err
}

func (c *Client) fetchGitLabMRs(ctx context.Context, apiURL string, headers map[string]string) ([]PullRequest, error) {
	var glMRs []struct {
		IID          int    `json:"iid"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		SHA          string `json:"sha"`
	}

	if err := c.doJSONRequest(ctx, apiURL, headers, &glMRs); err != nil {
		return nil, fmt.Errorf("gitlab api request failed: %w", err)
	}

	prs := make([]PullRequest, len(glMRs))
	for i, glMR := range glMRs {
		prs[i] = PullRequest{
			Number:     glMR.IID,
			Branch:     glMR.SourceBranch,
			HeadSHA:    glMR.SHA,
			BaseBranch: glMR.TargetBranch,
		}
	}

	return prs, nil
}

func (c *Client) fetchBitbucketPRs(ctx context.Context, apiURL string, headers map[string]string) ([]PullRequest, error) {
	var bbPRs struct {
		Values []struct {
			ID     int `json:"id"`
			Source struct {
				Branch struct {
					Name string `json:"name"`
				} `json:"branch"`
				Commit struct {
					Hash string `json:"hash"`
				} `json:"commit"`
			} `json:"source"`
			Destination struct {
				Branch struct {
					Name string `json:"name"`
				} `json:"branch"`
			} `json:"destination"`
		} `json:"values"`
	}

	if err := c.doJSONRequest(ctx, apiURL, headers, &bbPRs); err != nil {
		return nil, fmt.Errorf("bitbucket api request failed: %w", err)
	}

	prs := make([]PullRequest, len(bbPRs.Values))
	for i, bbPR := range bbPRs.Values {
		prs[i] = PullRequest{
			Number:     bbPR.ID,
			Branch:     bbPR.Source.Branch.Name,
			HeadSHA:    bbPR.Source.Commit.Hash,
			BaseBranch: bbPR.Destination.Branch.Name,
		}
	}

	return prs, nil
}

func parseRepoURL(repoURL string, host string, allowNested bool) (owner string, repo string, err error) {
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// HTTPS
	httpsPrefix := fmt.Sprintf("https://%s/", host)
	if strings.HasPrefix(repoURL, httpsPrefix) {
		parts := strings.Split(strings.TrimPrefix(repoURL, httpsPrefix), "/")
		if len(parts) >= 2 {
			owner = parts[0]
			if allowNested {
				repo = strings.Join(parts[1:], "/")
			} else {
				repo = parts[1]
			}

			return owner, repo, nil
		}
	}

	// SSH
	sshPrefix := fmt.Sprintf("git@%s:", host)
	if strings.HasPrefix(repoURL, sshPrefix) {
		parts := strings.Split(strings.TrimPrefix(repoURL, sshPrefix), "/")
		if len(parts) >= 2 {
			owner = parts[0]
			if allowNested {
				repo = strings.Join(parts[1:], "/")
			} else {
				repo = parts[1]
			}

			return owner, repo, nil
		}
	}

	return "", "", fmt.Errorf("invalid %s URL: %s", host, repoURL)
}

func urlPathEscape(s string) string {
	return strings.ReplaceAll(s, "/", "%2F")
}
