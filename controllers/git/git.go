/*
Package git holds every implemention needed
to do various git operations.
This is a interface-first implemention.
*/
package git

import (
	"net/url"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-logr/logr"
)

// GitOperator defines the interface for git operations.
type GitOperator interface {
	Clone(repoURL string, secretRef []byte, directory string, logger logr.Logger) (err error)
	Poll(repoURL string, secretRef []byte, branch string, directory string, logger logr.Logger) (changes bool, err error)
	Hash(repoURL string, secretRef []byte, branch string, logger logr.Logger) (hash string, err error)
	CheckAccess(repoURL string, secretRef []byte, logger logr.Logger) (accessible bool, requiresAuth bool, err error)
}

// GitImplementer implements the GitOperator interface.
type GitImplementer struct{}

// Clone clones the given repoURLsitory to a local directory.
func (g *GitImplementer) Clone(repoURL string, secretRef []byte, directory string, logger logr.Logger) (err error) {
	var auth transport.AuthMethod

	logger.Info("Creating directory")
	err = os.Mkdir(directory, 0755)
	if err != nil {
		logger.Error(err, "Failed to create directory", "directory", directory)

		return err
	}

	// Check if directory is empty.
	logger.Info("Checking directory is empty")
	if directory == "" {
		logger.Error(err, "Directory is empty", "directory", directory)

		return err
	}

	if secretRef != nil {
		auth, err = getSSHAuth(secretRef, logger)
		if err != nil {
			logger.Error(err, "Failed to run getSSHAuth")

			return err
		}
	}

	logger.Info("Plain Cloning repoURL")
	_, err = git.PlainClone(directory, false, &git.CloneOptions{
		URL:   repoURL,
		Auth:  auth,
		Depth: 1,
	})
	if err != nil {
		logger.Error(err, "Failed to clone repoURL", "repoURL", repoURL)

		return err
	}

	return err
}

// Poll polls for changes for the given remote git repoURLsitory. Returns true, if current local commit hash and remote hash are not equal.
func (g *GitImplementer) Poll(repoURL string, secretRef []byte, branch string, directory string, logger logr.Logger) (changes bool, err error) {
	// Defaults to false. We only change to true if there is a difference between the hashes.
	changes = false

	// Check if directory is empty.
	if directory == "" {
		logger.Error(err, "Directory is empty", "directory", directory)

		return changes, err
	}

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

func (g *GitImplementer) Hash(repoURL string, secretRef []byte, branch string, logger logr.Logger) (hash string, err error) {
	if isURL(repoURL) {
		return g.remoteHash(repoURL, secretRef, branch, logger)
	}

	return g.localHash(repoURL, logger)
}

func (g *GitImplementer) CheckAccess(repoURL string, secretRef []byte, logger logr.Logger) (accessible bool, requiresAuth bool, err error) {
	remoteRepo := git.NewRemote(nil, &config.RemoteConfig{
		URLs: []string{repoURL},
	})

	// publicRepository
	_, err = remoteRepo.List(&git.ListOptions{
		Auth: nil,
	})

	if err == nil {
		logger.Info("Repository is publicly accessible")
		accessible = true
		requiresAuth = false

		return accessible, requiresAuth, nil
	}

	auth, err := getSSHAuth(secretRef, logger)
	if err != nil {
		logger.Error(err, "Failed to run getSSHAuth")
		accessible = false
		requiresAuth = true

		return accessible, requiresAuth, err
	}

	// privateRepository
	_, err = remoteRepo.List(&git.ListOptions{
		Auth: auth,
	})

	if err == nil {
		logger.Info("Repository is privatly accessible")
		accessible = true
		requiresAuth = true

		return accessible, requiresAuth, nil
	}

	return accessible, requiresAuth, err
}

func getSSHAuth(secretRef []byte, logger logr.Logger) (auth transport.AuthMethod, err error) {
	if len(secretRef) == 0 {
		logger.Error(err, "secretRef reference is empty")
	}

	auth, err = ssh.NewPublicKeys("git", secretRef, "")
	if err != nil {
		logger.Error(err, "Failed on retrieve the token from the tokenContent")

		return auth, err
	}

	return auth, err
}

// localHash retrieves the HEAD commit hash from a local repository.
func (g *GitImplementer) localHash(path string, logger logr.Logger) (hash string, err error) {
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

func (g *GitImplementer) remoteHash(repoURL string, secretRef []byte, branch string, logger logr.Logger) (hash string, err error) {
	if branch == "" {
		logger.Error(err, "Branch is empty")

		return hash, err
	}

	auth, err := getSSHAuth(secretRef, logger)
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
	if repoURL == "" {
		return false
	}
	parsedURL, err := url.ParseRequestURI(repoURL)
	if err == nil && parsedURL.Scheme != "" {
		return true
	}

	if strings.Contains(repoURL, "@") && strings.Contains(repoURL, ":") {
		return true
	}

	return false
}
