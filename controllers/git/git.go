/*
Package git holds every implementation needed
to do various git operations.
*/
package git

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-logr/logr"
)

type (
	authType string
	// Provider represents a Git provider type.
	providerType string
)

const (
	// authTypeUnknown indicates we can't determine the auth type from the URL.
	authTypeUnknown authType = "unknown"
	// authTypeSSH indicates the URL is for SSH authentication.
	authTypeSSH authType = "ssh"
	// authTypeHTTP indicates the URL is for HTTP/S authentication.
	authTypeHTTP authType = "http"
	// ProviderGitHub defines the Provider type of GitHub.
	ProviderGitHub providerType = "github"
	// ProviderGitLab defines the Provider type of GitLab.
	ProviderGitLab providerType = "gitlab"
	// ProviderBitbucket defines the Provider type of BitBucket.
	ProviderBitbucket providerType = "bitbucket"
)

type PullRequest struct {
	ID         int    `json:"id"`
	Number     int    `json:"number"`
	Branch     string `json:"branch"`
	HeadSHA    string `json:"head_sha"`
	BaseBranch string `json:"base_branch"`
}

// Client implements the ProviderClient interface for various Git providers.
type Client struct {
	httpClientContainer
	provider    providerType
	host        string
	allowNested bool
}

// Operator defines the interface for git operations.
type Operator interface {
	Clone(repoURL string, secretRef []byte, branch string, directory string, logger logr.Logger) (err error)
	Poll(repoURL string, secretRef []byte, branch string, directory string, logger logr.Logger) (changes bool, err error)
	Hash(repoURL string, secretRef []byte, branch string, logger logr.Logger) (hash string, err error)
	CheckAccess(repoURL string, secretRef []byte, logger logr.Logger) (accessible bool, requiresAuth bool, err error)
	ListPullRequests(ctx context.Context, repoURL string, secretRef []byte) (prs []PullRequest, err error)
}

type ProviderClient interface {
	ListPullRequests(ctx context.Context, repoURL string, secretRef []byte) (prs []PullRequest, err error)
}

// Implementer implements the GitOperator interface.
type Implementer struct{}

// Clone clones the given repository to a local directory.
func (g *Implementer) Clone(repoURL string, secretRef []byte, branch string, directory string, logger logr.Logger) (err error) {
	var auth transport.AuthMethod

	logger.Info("Starting to clone git repository", "repoURL", repoURL, "branch", branch, "directory", directory)

	err = os.MkdirAll(directory, 0755)
	if err != nil {
		logger.Error(err, "Failed to create directory", "directory", directory)

		return err
	}

	if secretRef != nil {
		auth, err = getAuth(repoURL, secretRef, logger)
		if err != nil {
			logger.Error(err, "Failed to run getAuth")

			return err
		}
	}

	_, err = git.PlainClone(directory, false, &git.CloneOptions{
		URL:           repoURL,
		Auth:          auth,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Depth:         1,
	})
	if err != nil {
		logger.Error(err, "Failed to clone git repository", "repoURL", repoURL, "directory", directory)

		return err
	}
	logger.Info("Successfully cloned git repository", "repoURL", repoURL, "directory", directory)

	return err
}

// Poll polls for changes for the given remote git repository. Returns true, if current local commit hash and remote hash are not equal.
func (g *Implementer) Poll(repoURL string, secretRef []byte, branch string, directory string, logger logr.Logger) (changes bool, err error) {
	// Defaults to false. We only change to true if there is a difference between the hashes.
	changes = false

	// Get hash from local repoURL.
	localHash, err := g.Hash(directory, nil, branch, logger)
	if err != nil {
		logger.Error(err, "Failed to get hash", "repoURL", repoURL, "directory", directory)

		return changes, err
	}

	// Get Hash from remote repoURL
	remoteHash, err := g.Hash(repoURL, secretRef, branch, logger)
	if err != nil {
		logger.Error(err, "Failed to get hash", "repoURL", repoURL, "directory", directory)

		return changes, err
	}

	if localHash != remoteHash {
		changes = true
	}

	return changes, err
}

func (g *Implementer) Hash(repoURL string, secretRef []byte, branch string, logger logr.Logger) (hash string, err error) {
	if isURL(repoURL) {
		return g.remoteHash(repoURL, secretRef, branch, logger)
	}

	return g.localHash(repoURL, logger)
}

func (g *Implementer) CheckAccess(repoURL string, secretRef []byte, logger logr.Logger) (accessible bool, requiresAuth bool, err error) {
	remoteRepo := git.NewRemote(nil, &config.RemoteConfig{
		URLs: []string{repoURL},
	})

	// publicRepository
	_, err = remoteRepo.List(&git.ListOptions{
		Auth: nil,
	})

	if err == nil {
		accessible = true
		requiresAuth = false

		return accessible, requiresAuth, nil
	}

	auth, err := getAuth(repoURL, secretRef, logger)
	if err != nil {
		logger.Error(err, "Failed to run getAuth")
		accessible = false
		requiresAuth = true

		return accessible, requiresAuth, err
	}

	// privateRepository
	_, err = remoteRepo.List(&git.ListOptions{
		Auth: auth,
	})

	if err == nil {
		accessible = true
		requiresAuth = true

		return accessible, requiresAuth, nil
	}

	return accessible, requiresAuth, err
}

func getAuth(repoURL string, secretRef []byte, logger logr.Logger) (auth transport.AuthMethod, err error) {
	if len(secretRef) == 0 {
		logger.Error(err, "secretRef reference is empty")
		// conditions.Set(cdk8sAppProxy, metav1.Condition{
		// 		Type: clusterv1.AvailableCondition,
		// 		Status: metav1.ConditionFalse,
		// 		Reason: "Failed",
		// 		Message: "Failed to clone Git Repository",
		// 	})

		return auth, err
	}

	urlType := getURLType(repoURL)

	switch urlType {
	case authTypeHTTP:
		logger.Info("Using HTTP Basic Auth (PAT) for URL", "url", repoURL)
		auth = &http.BasicAuth{
			Username: "oauth2",
			Password: string(secretRef),
		}

		return auth, err
	case authTypeSSH:
		logger.Info("Using SSH Key Auth for URL", "url", repoURL)
		auth, err = ssh.NewPublicKeys("git", secretRef, "")
		if err != nil {
			logger.Error(err, "Failed on process the SSH token for URL", "url", repoURL)

			return auth, err
		}
	case authTypeUnknown:
		logger.Info("unknown type")

		fallthrough
	default:
		logger.Error(err, "unknown or unsupported URL scheme for auth")

		return auth, err
	}

	return auth, err
}

// getURLType checks the kind of the given URL, and returns the type of the auth Method.
func getURLType(repoURL string) authType {
	if strings.HasPrefix(repoURL, "http://") || strings.HasPrefix(repoURL, "https://") {
		return authTypeHTTP
	}

	// Covers ssh://user@host/repo.git
	if strings.HasPrefix(repoURL, "ssh://") {
		return authTypeSSH
	}

	// Covers git@host:repo.git
	if strings.Contains(repoURL, "@") && strings.Contains(repoURL, ":") && !strings.HasPrefix(repoURL, "http") {
		return authTypeSSH
	}

	return authTypeUnknown
}

// localHash retrieves the HEAD commit hash from a local repository.
func (g *Implementer) localHash(path string, logger logr.Logger) (hash string, err error) {
	localRepo, err := git.PlainOpen(path)
	if err != nil {
		logger.Error(err, "failed to open local repo")

		return hash, err
	}

	headRef, err := localRepo.Head()
	if err != nil {
		logger.Error(err, "failed to get head of local repo")

		return hash, err
	}
	hash = headRef.Hash().String()

	return hash, err
}

func (g *Implementer) remoteHash(repoURL string, secretRef []byte, branch string, logger logr.Logger) (hash string, err error) {
	auth, err := getAuth(repoURL, secretRef, logger)
	if err != nil {
		return hash, err
	}

	remoteRepo := git.NewRemote(nil, &config.RemoteConfig{
		URLs: []string{repoURL},
	})

	refs, err := remoteRepo.List(&git.ListOptions{
		Auth: auth,
	})
	if err != nil {
		logger.Error(err, "Failed to list remote repo")

		return hash, err
	}

	refName := plumbing.NewBranchReferenceName(branch)
	for _, ref := range refs {
		if ref.Name() == refName {
			hash = ref.Hash().String()

			return hash, err
		}
	}

	return hash, err
}

// isURL checks if the given string is a valid URL.
func isURL(repoURL string) bool {
	parsedURL, err := url.ParseRequestURI(repoURL)
	if err == nil && parsedURL.Scheme != "" {
		return true
	}

	if strings.Contains(repoURL, "@") && strings.Contains(repoURL, ":") {
		return true
	}

	return false
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
