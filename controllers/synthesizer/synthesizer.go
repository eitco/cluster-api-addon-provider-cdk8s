package synthesizer

import (
	"bytes"
	"context"
	"github.com/go-logr/logr"
	"io/fs"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	addonsv1alpha1 "github.com/eitco/cluster-api-addon-provider-cdk8s/api/v1alpha1"
)

type ApplicationType string

const (
	cdk8sGo         ApplicationType = "go"
	cdk8sTypescript ApplicationType = "typescript"
	cdk8sPython     ApplicationType = "python"
)

type KustomizationFile struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace,omitempty"`
	} `yaml:"metadata"`
	Resources []string `yaml:"resources"`
}

type Synthesizer interface {
	Synthesize(directory string, cdk8sAppProxy *addonsv1alpha1.Cdk8sAppProxy, logger logr.Logger, ctx context.Context) (parsedManifests []*unstructured.Unstructured, err error)
}

// Implementer implements the Synthesizer method.
type Implementer struct{}

func (i *Implementer) Synthesize(directory string, cdk8sAppProxy *addonsv1alpha1.Cdk8sAppProxy, logger logr.Logger, ctx context.Context) (parsedManifests []*unstructured.Unstructured, err error) {
	apiPath := filepath.Join(directory, cdk8sAppProxy.Spec.GitRepository.Path)

	kind := cdk8sType(directory, logger)

	if kind == string(cdk8sTypescript) {
		npmInstall := exec.CommandContext(ctx, "npm", "install")
		npmInstall.Dir = apiPath
		output, err := npmInstall.CombinedOutput()
		if err != nil {
			logger.Error(err, "npm installation failed", "msg:", string(output))
		}
	}

	synth := exec.CommandContext(ctx, "cdk8s", "synth")
	synth.Dir = apiPath
	if err := synth.Run(); err != nil {
		logger.Error(err, "Failed to synth cdk8s application")

		return parsedManifests, err
	}

	foundManifests, err := findManifests(apiPath, logger)
	if err != nil {
		logger.Error(err, "Failed to find manifests in directory")

		return parsedManifests, err
	}

	if len(foundManifests) > 0 && isKustomization(filepath.Base(foundManifests[0])) {
		var allManifests []string

		for _, kustomizationFile := range foundManifests {
			manifestPaths, err := parseKustomization(kustomizationFile, logger)

			if err != nil {
				logger.Error(err, "Failed to parse kustomization file")

				return parsedManifests, err
			}
			allManifests = append(allManifests, manifestPaths...)
		}

		parsedManifests, err = parseManifests(allManifests, logger)
		if err != nil {
			logger.Error(err, "Failed to parse kustomization manifests")

			return parsedManifests, err
		}
	} else {
		parsedManifests, err = parseManifests(foundManifests, logger)
		if err != nil {
			logger.Error(err, "Failed to parse manifests")
		}
	}

	return parsedManifests, err
}

// findManifests finds manifests in the specified directory.
func findManifests(directory string, logger logr.Logger) (manifests []string, err error) {
	distPath := filepath.Join(directory, "dist")

	var kustomizeFiles []string
	var regularManifests []string

	err = filepath.WalkDir(distPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			fileName := d.Name()
			if strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml") {
				if isKustomization(fileName) {
					kustomizeFiles = append(kustomizeFiles, path)
				} else if !isKustomization(fileName) {
					regularManifests = append(regularManifests, path)
				}
			}
		}

		return err
	})
	if err != nil {
		logger.Error(err, "Failed to walk dist directory")

		return manifests, err
	}

	if len(kustomizeFiles) > 0 {
		manifests = kustomizeFiles
	} else {
		manifests = regularManifests
	}

	return manifests, err
}

// parseManifests parses manifests from the specified directory.
func parseManifests(manifests []string, logger logr.Logger) (parsedResources []*unstructured.Unstructured, err error) {
	for _, manifest := range manifests {
		manifestContent, err := os.ReadFile(manifest)
		if err != nil {
			logger.Error(err, "Failed to read manifest file.")

			return parsedResources, err
		}

		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifestContent), 1024)

		for {
			var rawObj runtime.RawExtension
			if err = decoder.Decode(&rawObj); err != nil {
				if err.Error() == "EOF" {
					break
				}
				logger.Error(err, "Failed to decode YAML from manifest file")

				return parsedResources, err
			}

			if rawObj.Raw == nil {
				continue
			}

			u := &unstructured.Unstructured{}
			if _, _, err := unstructured.UnstructuredJSONScheme.Decode(rawObj.Raw, nil, u); err != nil {
				logger.Error(err, "Failed to decode RawExtension to Unstructured")

				return parsedResources, err
			}

			parsedResources = append(parsedResources, u)
		}
	}

	return parsedResources, err
}

// parseKustomization reads a kustomization file and returns the referenced manifest files.
func parseKustomization(kustomizationPath string, logger logr.Logger) (manifestPaths []string, err error) {
	content, err := os.ReadFile(kustomizationPath)
	if err != nil {
		logger.Error(err, "Failed to read kustomization file", "path", kustomizationPath)

		return manifestPaths, err
	}

	var kustomization KustomizationFile
	if err := yaml.Unmarshal(content, &kustomization); err != nil {
		logger.Error(err, "Failed to parse kustomization file", "path", kustomizationPath)

		return manifestPaths, err
	}

	// Get the directory containing the kustomization file
	kustomizeDir := filepath.Dir(kustomizationPath)

	// Resolve resource paths relative to the kustomization file directory
	for _, resource := range kustomization.Resources {
		resourcePath := filepath.Join(kustomizeDir, resource)
		manifestPaths = append(manifestPaths, resourcePath)
	}

	return manifestPaths, nil
}

func cdk8sType(directory string, logger logr.Logger) (kind string) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		logger.Error(err, "Failed to read directory")

		return kind
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		switch {
		case strings.HasSuffix(fileName, ".go"):
			kind = string(cdk8sGo)

			return kind
		case strings.HasSuffix(fileName, ".ts"):
			kind = string(cdk8sTypescript)

			return kind
		case strings.HasSuffix(fileName, ".py"):
			kind = string(cdk8sPython)

			return kind
		}
	}

	return kind
}

var kustomizationNames = []string{"kustomization.yaml", "kustomization.yml", "kustomization.k8s.yaml", "Kustomization"}

// isKustomization checks if the given file name matches any known kustomization file names.
func isKustomization(path string) bool {
	for _, kustomization := range kustomizationNames {
		if path == kustomization {
			return true
		}
	}

	return false
}
