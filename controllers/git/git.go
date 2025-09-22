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
}

// GitImplementer implements the GitOperator interface.
type GitImplementer struct{}

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

// Clone clones the given repoURLsitory to a local directory.
func (g *GitImplementer) Clone(repoURL string, secretRef []byte, directory string, logger logr.Logger) (err error) {
	logger.Info("Creating directory")
	err = os.Mkdir(directory, 0755)
	if err != nil {
		logger.Error(err, "Failed to create directory", "directory", directory)

		return err
	}

	// Check if repoURL and directory are empty.
	logger.Info("Checking if repoURL and or directory is empty")
	if empty(repoURL, directory) {
		logger.Error(err, "repoURL and or directory is empty", "repoURL", repoURL, "directory", directory)

		return err
	}

	auth, err := getSSHAuth(secretRef, logger)
	if err != nil {
		logger.Error(err, "Failed to run getSSHAuth")
	}

	logger.Info("Plain Cloning repoURL")
	_, err = git.PlainClone(directory, false, &git.CloneOptions{
		URL: repoURL,
		Auth: auth, 
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

	// Check if repoURL and directory are empty.
	if empty(repoURL, directory) {
		logger.Error(err, "repoURL and or directory is empty", "repoURL", repoURL, "directory", directory)

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
// Hash retrieves the hash of the given repoURLsitory.
// func (g *GitImplementer) Hash(repoURL string, secretRef []byte, branch string, logger logr.Logger) (hash string, err error) {
// 	auth, err = ssh.NewPublicKeys("git", secretRef, "")
// 	if err != nil {
// 		logger.Error(err, "Failed to retrieve the token from the tokenContent")
//
// 		return hash, err
// 	}
//
// 	pkAuth, ok := auth.(*ssh.PublicKeys)
// 	if !ok {
// 		logger.Error(err, "pkAuth error")
// 	}
//
// 	pkAuth.HostKeyCallback = gossh.InsecureIgnoreHostKey()
//
// 	switch {
// 	case isURL(repoURL):
// 		remoterepoURL := git.NewRemote(nil, &config.RemoteConfig{
// 			URLs: []string{repoURL},
// 			Name: "origin",
// 		})
//
// 		refs, err := remoterepoURL.List(&git.ListOptions{
// 			Auth: auth,
// 		})
// 		if err != nil {
// 			logger.Error(err, "Failed to list remote repoURL", "repoURL", repoURL)
//
// 			return hash, err
// 		}
//
// 		refName := plumbing.NewBranchReferenceName(branch)
// 		for _, ref := range refs {
// 			if ref.Name() == refName {
// 				return ref.Hash().String(), err
// 			}
// 		}
//
// 		return hash, err
// 	case !isURL(repoURL):
// 		localrepoURL, err := git.PlainOpen(repoURL)
// 		if err != nil {
// 			logger.Error(err, "Failed to open local repoURL", "repoURL", repoURL)
//
// 			return hash, err
// 		}
//
// 		headRef, err := localrepoURL.Head()
// 		if err != nil {
// 			logger.Error(err, "failed to get head for local git repoURL", "repoURL", repoURL)
//
// 			return hash, err
// 		}
//
// 		hash = headRef.Hash().String()
// 		if hash == "" {
// 			logger.Error(err, "failed to get hash for local git repoURL", "repoURL", repoURL)
//
// 			return hash, err
// 		}
//
// 		return hash, err
// 	}
//
// 	return hash, err
// }

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
	// if parsedURL.Scheme != "" {
	// 	return true
	// } else {
	// 	return false
	// }
	return false
}

// empty checks if the repoURL and directory strings are empty.
func empty(repoURL string, directory string) bool {
	if repoURL == "" || directory == "" {
		return true
	}

	return false
}
