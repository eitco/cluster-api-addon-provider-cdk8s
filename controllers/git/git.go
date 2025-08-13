package git

import (
	"net/url"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-logr/logr"
)

// GitOperator defines the interface for git operations.
type GitOperator interface {
	Clone(repoUrl string, dir string, logger logr.Logger) (err error)
	Poll(repo string, branch string, directory string, logger logr.Logger) (changes bool, err error)
	Hash(repo string, branch string, logger logr.Logger) (hash string, err error)
}

// GitImplementer implements the GitOperator interface.
type GitImplementer struct{}

// Clone clones the given repository to a local directory.
func (g *GitImplementer) Clone(repoUrl string, directory string, logger logr.Logger) (err error) {
	err = os.Mkdir(directory, 0755)
	if err != nil {
		logger.Error(err, "Failed to create directory", "directory", directory)

		return err
	}

	// Check if repo and directory are empty.
	if empty(repoUrl, directory) {
		logger.Error(err, "repo and or directory is empty", "repoUrl", repoUrl, "directory", directory)

		return err
	}

	_, err = git.PlainClone(directory, false, &git.CloneOptions{
		URL: repoUrl,
	})
	if err != nil {
		logger.Error(err, "Failed to clone repo", "repo", repoUrl)

		return err
	}

	return err
}

// Poll polls for changes for the given remote git repository. Returns true, if current local commit hash and remote hash are not equal.
func (g *GitImplementer) Poll(repo string, branch string, directory string, logger logr.Logger) (changes bool, err error) {
	// Defaults to false. We only change to true if there is a difference between the hashes.
	changes = false

	// Check if repo and directory are empty.
	if empty(repo, directory) {
		logger.Error(err, "repo and or directory is empty", "repo", repo, "directory", directory)

		return changes, err
	}

	// Get hash from local repo.
	localHash, err := g.Hash(directory, branch, logger)
	if err != nil {
		logger.Error(err, "Failed to get hash", "repo", repo, "directory", directory)

		return changes, err
	}

	// Get Hash from remote repo
	remoteHash, err := g.Hash(repo, branch, logger)
	if err != nil {
		logger.Error(err, "Failed to get hash", "repo", repo, "directory", directory)

		return changes, err
	}

	if localHash != remoteHash {
		changes = true
	}

	return changes, err
}

// Hash retrieves the hash of the given repository.
func (g *GitImplementer) Hash(repo string, branch string, logger logr.Logger) (hash string, err error) {
	switch {
	case isUrl(repo):
		remoterepo := git.NewRemote(nil, &config.RemoteConfig{
			URLs: []string{repo},
			Name: "origin",
		})

		refs, err := remoterepo.List(&git.ListOptions{})
		if err != nil {
			logger.Error(err, "Failed to list remote repo", "repo", repo)

			return hash, err
		}

		refName := plumbing.NewBranchReferenceName(branch)
		for _, ref := range refs {
			if ref.Name() == refName {
				return ref.Hash().String(), err
			}
		}

		return hash, err
	case !isUrl(repo):
		localRepo, err := git.PlainOpen(repo)
		if err != nil {
			logger.Error(err, "Failed to open local repo", "repo", repo)

			return hash, err
		}

		headRef, err := localRepo.Head()
		if err != nil {
			logger.Error(err, "failed to get head for local git repo", "repo", repo)

			return hash, err
		}

		hash = headRef.Hash().String()
		if hash == "" {
			logger.Error(err, "failed to get hash for local git repo", "repo", repo)

			return hash, err
		}

		return hash, err
	}

	return hash, err
}

// isUrl checks if the given string is a valid URL.
func isUrl(repo string) bool {
	if repo == "" {
		return false
	}
	parsedUrl, err := url.ParseRequestURI(repo)
	if err != nil {
		return false
	}

	if parsedUrl.Scheme != "" {
		return true
	} else {
		return false
	}
}

// empty checks if the repo and directory strings are empty.
func empty(repo string, directory string) bool {
	if repo == "" || directory == "" {
		return true
	}

	return false
}
