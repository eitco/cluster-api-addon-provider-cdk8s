package synthesizer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestFindManifests(t *testing.T) {
	logger := logr.Discard()

	t.Run("should find yaml and yml files", func(t *testing.T) {
		tempDir := t.TempDir()
		distDir := filepath.Join(tempDir, "dist")
		assert.NoError(t, os.Mkdir(distDir, 0755))

		_, err := os.Create(filepath.Join(distDir, "test1.yaml"))
		assert.NoError(t, err)
		_, err = os.Create(filepath.Join(distDir, "test2.yml"))
		assert.NoError(t, err)
		_, err = os.Create(filepath.Join(distDir, "test3.txt"))
		assert.NoError(t, err)

		manifests, err := findManifests(tempDir, logger)
		assert.NoError(t, err)
		assert.Len(t, manifests, 2)
	})

	t.Run("should return error for non-existent directory", func(t *testing.T) {
		tempDir := t.TempDir()
		_, err := findManifests(filepath.Join(tempDir, "non-existent"), logger)
		assert.Error(t, err)
	})

	t.Run("should handle empty dist directory", func(t *testing.T) {
		tempDir := t.TempDir()
		distDir := filepath.Join(tempDir, "dist")
		assert.NoError(t, os.Mkdir(distDir, 0755))

		manifests, err := findManifests(tempDir, logger)
		assert.NoError(t, err)
		assert.Len(t, manifests, 0)
	})
}

func TestParseManifests(t *testing.T) {
	logger := logr.Discard()

	t.Run("should parse a valid manifest", func(t *testing.T) {
		content := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
`
		tempFile, err := os.CreateTemp("", "manifest-*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())
		_, err = tempFile.WriteString(content)
		assert.NoError(t, err)
		tempFile.Close()

		manifests := []string{tempFile.Name()}
		parsed, err := parseManifests(manifests, logger)
		assert.NoError(t, err)
		assert.Len(t, parsed, 1)
		assert.Equal(t, "ConfigMap", parsed[0].GetKind())
		assert.Equal(t, "test-cm", parsed[0].GetName())
	})

	t.Run("should parse multi-document yaml", func(t *testing.T) {
		content := `
apiVersion: v1
kind: Service
metadata:
  name: my-service
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
`
		tempFile, err := os.CreateTemp("", "manifest-*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())
		_, err = tempFile.WriteString(content)
		assert.NoError(t, err)
		tempFile.Close()

		manifests := []string{tempFile.Name()}
		parsed, err := parseManifests(manifests, logger)
		assert.NoError(t, err)
		assert.Len(t, parsed, 2)
		assert.Equal(t, "Service", parsed[0].GetKind())
		assert.Equal(t, "Deployment", parsed[1].GetKind())
	})

	t.Run("should return error for invalid yaml", func(t *testing.T) {
		content := `
apiVersion: v1
kind: ConfigMap
  name: test-cm
`
		tempFile, err := os.CreateTemp("", "manifest-*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())
		_, err = tempFile.WriteString(content)
		assert.NoError(t, err)
		tempFile.Close()

		manifests := []string{tempFile.Name()}
		_, err = parseManifests(manifests, logger)
		assert.Error(t, err)
	})

	t.Run("should return error for non-existent file", func(t *testing.T) {
		manifests := []string{"non-existent-file.yaml"}
		_, err := parseManifests(manifests, logger)
		assert.Error(t, err)
	})
}

func TestCdk8sType(t *testing.T) {
	logger := logr.Discard()

	t.Run("should detect Go application", func(t *testing.T) {
		tempDir := t.TempDir()
		_, err := os.Create(filepath.Join(tempDir, "main.go"))
		assert.NoError(t, err)

		kind := cdk8sType(tempDir, logger)
		assert.Equal(t, "go", kind)
	})

	t.Run("should detect TypeScript application", func(t *testing.T) {
		tempDir := t.TempDir()
		_, err := os.Create(filepath.Join(tempDir, "main.ts"))
		assert.NoError(t, err)

		kind := cdk8sType(tempDir, logger)
		assert.Equal(t, "typescript", kind)
	})

	t.Run("should detect Python application", func(t *testing.T) {
		tempDir := t.TempDir()
		_, err := os.Create(filepath.Join(tempDir, "main.py"))
		assert.NoError(t, err)

		kind := cdk8sType(tempDir, logger)
		assert.Equal(t, "python", kind)
	})

	t.Run("should return empty string for unknown type", func(t *testing.T) {
		tempDir := t.TempDir()
		_, err := os.Create(filepath.Join(tempDir, "readme.txt"))
		assert.NoError(t, err)

		kind := cdk8sType(tempDir, logger)
		assert.Equal(t, "", kind)
	})

	t.Run("should return empty string for empty directory", func(t *testing.T) {
		tempDir := t.TempDir()

		kind := cdk8sType(tempDir, logger)
		assert.Equal(t, "", kind)
	})

	t.Run("should prioritize Go over other types", func(t *testing.T) {
		tempDir := t.TempDir()
		_, err := os.Create(filepath.Join(tempDir, "main.go"))
		assert.NoError(t, err)
		_, err = os.Create(filepath.Join(tempDir, "main.ts"))
		assert.NoError(t, err)

		kind := cdk8sType(tempDir, logger)
		assert.Equal(t, "go", kind)
	})

	t.Run("should handle non-existent directory", func(t *testing.T) {
		kind := cdk8sType("/non/existent/directory", logger)
		assert.Equal(t, "", kind)
	})
}

func TestParseKustomization(t *testing.T) {
	logger := logr.Discard()

	t.Run("should parse kustomization file and return resource paths", func(t *testing.T) {
		content := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
metadata:
  name: kustomization
  namespace: default
resources:
  - headlamp-deployment.k8s.yaml
  - nginx-deployment.k8s.yaml`

		tempFile, err := os.CreateTemp("", "kustomization.yaml")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())
		_, err = tempFile.WriteString(content)
		assert.NoError(t, err)
		tempFile.Close()

		manifestPaths, err := parseKustomization(tempFile.Name(), logger)
		assert.NoError(t, err)
		assert.Len(t, manifestPaths, 2)

		expectedDir := filepath.Dir(tempFile.Name())
		assert.Equal(t, filepath.Join(expectedDir, "headlamp-deployment.k8s.yaml"), manifestPaths[0])
		assert.Equal(t, filepath.Join(expectedDir, "nginx-deployment.k8s.yaml"), manifestPaths[1])
	})

	t.Run("should handle empty resources list", func(t *testing.T) {
		content := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
metadata:
  name: kustomization
resources: []`

		tempFile, err := os.CreateTemp("", "kustomization.yaml")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())
		_, err = tempFile.WriteString(content)
		assert.NoError(t, err)
		tempFile.Close()

		manifestPaths, err := parseKustomization(tempFile.Name(), logger)
		assert.NoError(t, err)
		assert.Len(t, manifestPaths, 0)
	})

	t.Run("should return error for non-existent file", func(t *testing.T) {
		_, err := parseKustomization("/non/existent/kustomization.yaml", logger)
		assert.Error(t, err)
	})

	t.Run("should return error for invalid YAML", func(t *testing.T) {
		content := `invalid yaml content: [}`

		tempFile, err := os.CreateTemp("", "kustomization.yaml")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())
		_, err = tempFile.WriteString(content)
		assert.NoError(t, err)
		tempFile.Close()

		_, err = parseKustomization(tempFile.Name(), logger)
		assert.Error(t, err)
	})
}
func TestImplementer_Synthesize(t *testing.T) {
	logger := logr.Discard()
	impl := &Implementer{}

	t.Run("should return error if directory does not exist", func(t *testing.T) {
		parsed, err := impl.Synthesize("/non/existent/dir", logger)
		assert.Error(t, err)
		assert.Nil(t, parsed)
	})

	t.Run("should return error if cdk8s synth fails", func(t *testing.T) {
		// Create a temp dir with a Go file so cdk8sType returns "go"
		tempDir := t.TempDir()
		_, err := os.Create(filepath.Join(tempDir, "main.go"))
		assert.NoError(t, err)

		// No cdk8s binary, so synth should fail
		parsed, err := impl.Synthesize(tempDir, logger)
		assert.Error(t, err)
		assert.Nil(t, parsed)
	})

	t.Run("should parse manifests if present after synth", func(t *testing.T) {
		tempDir := t.TempDir()
		distDir := filepath.Join(tempDir, "dist")
		assert.NoError(t, os.Mkdir(distDir, 0755))
		manifestContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
`
		manifestPath := filepath.Join(distDir, "test.yaml")
		assert.NoError(t, os.WriteFile(manifestPath, []byte(manifestContent), 0644))

		// Create a Go file so cdk8sType returns "go"
		_, err := os.Create(filepath.Join(tempDir, "main.go"))
		assert.NoError(t, err)

		// Create a fake cdk8s binary in PATH that just exits 0
		origPath := os.Getenv("PATH")
		fakeBinDir := t.TempDir()
		fakeCdk8s := filepath.Join(fakeBinDir, "cdk8s")
		assert.NoError(t, os.WriteFile(fakeCdk8s, []byte("#!/bin/sh\nexit 0\n"), 0755))
		os.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+origPath)
		defer os.Setenv("PATH", origPath)

		parsed, err := impl.Synthesize(tempDir, logger)
		assert.NoError(t, err)
		assert.Len(t, parsed, 1)
		assert.Equal(t, "ConfigMap", parsed[0].GetKind())
		assert.Equal(t, "test-cm", parsed[0].GetName())
	})

	t.Run("should parse kustomization manifests if kustomization file present", func(t *testing.T) {
		tempDir := t.TempDir()
		distDir := filepath.Join(tempDir, "dist")
		assert.NoError(t, os.Mkdir(distDir, 0755))

		// Write kustomization.yaml
		kustomContent := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
metadata:
  name: kustomization
resources:
  - cm.yaml
`
		kustomPath := filepath.Join(distDir, "kustomization.yaml")
		assert.NoError(t, os.WriteFile(kustomPath, []byte(kustomContent), 0644))

		// Write referenced manifest
		cmContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: kustom-cm
`
		cmPath := filepath.Join(distDir, "cm.yaml")
		assert.NoError(t, os.WriteFile(cmPath, []byte(cmContent), 0644))

		// Create a Go file so cdk8sType returns "go"
		_, err := os.Create(filepath.Join(tempDir, "main.go"))
		assert.NoError(t, err)

		// Fake cdk8s binary
		origPath := os.Getenv("PATH")
		fakeBinDir := t.TempDir()
		fakeCdk8s := filepath.Join(fakeBinDir, "cdk8s")
		assert.NoError(t, os.WriteFile(fakeCdk8s, []byte("#!/bin/sh\nexit 0\n"), 0755))
		os.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+origPath)
		defer os.Setenv("PATH", origPath)

		parsed, err := impl.Synthesize(tempDir, logger)
		assert.NoError(t, err)
		assert.Len(t, parsed, 1)
		assert.Equal(t, "ConfigMap", parsed[0].GetKind())
		assert.Equal(t, "kustom-cm", parsed[0].GetName())
	})

	t.Run("should run npm install for typescript projects", func(t *testing.T) {
		tempDir := t.TempDir()
		_, err := os.Create(filepath.Join(tempDir, "main.ts"))
		assert.NoError(t, err)

		// Fake npm and cdk8s binaries
		origPath := os.Getenv("PATH")
		fakeBinDir := t.TempDir()
		fakeNpm := filepath.Join(fakeBinDir, "npm")
		fakeCdk8s := filepath.Join(fakeBinDir, "cdk8s")
		assert.NoError(t, os.WriteFile(fakeNpm, []byte("#!/bin/sh\nexit 0\n"), 0755))
		assert.NoError(t, os.WriteFile(fakeCdk8s, []byte("#!/bin/sh\nexit 0\n"), 0755))
		os.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+origPath)
		defer os.Setenv("PATH", origPath)

		// Create dist dir and manifest
		distDir := filepath.Join(tempDir, "dist")
		assert.NoError(t, os.Mkdir(distDir, 0755))
		manifestContent := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: ts-cm
`
		manifestPath := filepath.Join(distDir, "test.yaml")
		assert.NoError(t, os.WriteFile(manifestPath, []byte(manifestContent), 0644))

		parsed, err := impl.Synthesize(tempDir, logger)
		assert.NoError(t, err)
		assert.Len(t, parsed, 1)
		assert.Equal(t, "ts-cm", parsed[0].GetName())
	})
}
