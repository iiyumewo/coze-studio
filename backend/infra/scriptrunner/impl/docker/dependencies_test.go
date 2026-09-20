/*
 * Copyright 2025 coze-dev Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 */

package docker

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-studio/backend/infra/scriptrunner"
)

func TestDependencyFingerprintIncludesRuntimeImage(t *testing.T) {
	first := dependencyFingerprint("requirements-hash", "sha256:image-a", "https://mirror-a/simple")
	second := dependencyFingerprint("requirements-hash", "sha256:image-b", "https://mirror-a/simple")
	require.NotEqual(t, first, second)
	require.NotEqual(t, first, dependencyFingerprint("requirements-hash", "sha256:image-a", "https://mirror-b/simple"))
	require.Equal(t, first, dependencyFingerprint("requirements-hash", "sha256:image-a", "https://mirror-a/simple"))
}

func TestManifestMatchesCompletionMarker(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".complete"), []byte("abc\n"), 0o640))
	require.True(t, manifestMatches(dir, "abc"))
	require.False(t, manifestMatches(dir, "def"))
}

func TestCleanupDependencyVersionsKeepsCurrentAndRollback(t *testing.T) {
	root := t.TempDir()
	versionsRoot := filepath.Join(root, "versions")
	require.NoError(t, os.MkdirAll(versionsRoot, 0o750))
	for _, name := range []string{"current-version", "rollback-version", "stale-version"} {
		dir := filepath.Join(versionsRoot, name)
		require.NoError(t, os.MkdirAll(dir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".complete"), []byte(name), 0o640))
	}
	// Make rollback-version the newest non-active version.
	require.NoError(t, os.Chtimes(filepath.Join(versionsRoot, "stale-version"), time.Now(), time.Now().Add(time.Minute)))
	require.NoError(t, os.Chtimes(filepath.Join(versionsRoot, "rollback-version"), time.Now(), time.Now().Add(2*time.Minute)))
	runner := &Runner{config: scriptrunner.Config{DependencyRoot: root, DependencyVersionsToKeep: 2}}
	runner.cleanupDependencyVersions(t.Context(), "current-version")
	require.DirExists(t, filepath.Join(versionsRoot, "current-version"))
	require.DirExists(t, filepath.Join(versionsRoot, "rollback-version"))
	require.NoDirExists(t, filepath.Join(versionsRoot, "stale-version"))
}

func TestPrepareDependenciesSkipsMissingRequirements(t *testing.T) {
	err := PrepareDependencies(t.Context(), scriptrunner.Config{
		DockerBinary:     "docker-binary-is-not-needed",
		RequirementsPath: filepath.Join(t.TempDir(), "missing-requirements.txt"),
	})
	require.NoError(t, err)
}

func TestPrepareDependenciesDockerIntegration(t *testing.T) {
	if os.Getenv("COZE_RUN_DOCKER_TESTS") != "1" {
		t.Skip("set COZE_RUN_DOCKER_TESTS=1 to run Docker integration tests")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker is not installed")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon is not available")
	}
	ensureDockerImage(t, defaultImage)

	requirementsPath := filepath.Join(t.TempDir(), "requirements.txt")
	require.NoError(t, os.WriteFile(requirementsPath, []byte("requests==2.32.4\n"), 0o640))
	dependencyRoot := t.TempDir()
	err := PrepareDependencies(t.Context(), scriptrunner.Config{
		DockerBinary:     "docker",
		DockerImage:      defaultImage,
		DependencyRoot:   dependencyRoot,
		RequirementsPath: requirementsPath,
		BuildNetwork:     "bridge",
	})
	require.NoError(t, err)
	current, err := filepath.EvalSymlinks(filepath.Join(dependencyRoot, "current"))
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(current, "requests", "__init__.py"))
	require.FileExists(t, filepath.Join(current, ".complete"))

	skillsRoot := t.TempDir()
	scriptPath := filepath.Join(skillsRoot, "dependency", "check.py")
	require.NoError(t, os.MkdirAll(filepath.Dir(scriptPath), 0o750))
	require.NoError(t, os.WriteFile(scriptPath, []byte("import requests\nprint(requests.__version__)\n"), 0o640))
	runner, err := NewRunner(scriptrunner.Config{
		SkillsRoot:          skillsRoot,
		WorkspaceRoot:       t.TempDir(),
		DependencyRoot:      dependencyRoot,
		DockerBinary:        "docker",
		DockerImage:         defaultImage,
		TimeoutSeconds:      20,
		DockerMemoryLimitMB: 256,
		DockerCPUs:          0.5,
		DockerPidsLimit:     64,
	})
	require.NoError(t, err)
	response, err := runner.Run(t.Context(), &scriptrunner.RunRequest{ScriptPath: "dependency/check.py", ConversationID: 1})
	require.NoError(t, err)
	require.Equal(t, "2.32.4\n", response.Stdout)
}
