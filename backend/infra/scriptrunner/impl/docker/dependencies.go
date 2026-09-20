/*
 * Copyright 2025 coze-dev Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 */

package docker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/coze-dev/coze-studio/backend/infra/scriptrunner"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
)

const (
	containerDependencyRoot = "/opt/skill-deps"
	dependencyBuildTimeout  = 15 * time.Minute
	dependencyBuildMemoryMB = 1024
	dependencyBuildCPUs     = 2.0
	dependencyBuildPids     = 128
)

type dependencyManifest struct {
	Fingerprint      string    `json:"fingerprint"`
	RequirementsHash string    `json:"requirements_hash"`
	RuntimeImage     string    `json:"runtime_image"`
	PipMirror        string    `json:"pip_mirror,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// PrepareDependencies makes the administrator-managed Python dependency set
// available to future one-shot execution containers. It is intentionally
// separate from Runner.Run so user code never triggers pip installation.
func PrepareDependencies(ctx context.Context, config scriptrunner.Config) error {
	if strings.TrimSpace(config.RequirementsPath) == "" {
		return nil
	}
	requirements, err := os.ReadFile(config.RequirementsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read Python requirements: %w", err)
	}
	if strings.TrimSpace(string(requirements)) == "" {
		return nil
	}
	runner, err := NewRunner(config)
	if err != nil {
		return err
	}
	return runner.prepareDependencies(ctx)
}

func (r *Runner) prepareDependencies(ctx context.Context) error {
	requirementsPath := strings.TrimSpace(r.config.RequirementsPath)
	if requirementsPath == "" {
		return nil
	}
	requirementsPath, err := filepath.Abs(requirementsPath)
	if err != nil {
		return fmt.Errorf("resolve requirements path: %w", err)
	}
	requirements, err := os.ReadFile(requirementsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read Python requirements: %w", err)
	}
	if strings.TrimSpace(string(requirements)) == "" {
		return nil
	}

	if err = os.MkdirAll(r.config.DependencyRoot, 0o750); err != nil {
		return fmt.Errorf("create dependency root: %w", err)
	}
	versionsRoot := filepath.Join(r.config.DependencyRoot, "versions")
	if err = os.MkdirAll(versionsRoot, 0o750); err != nil {
		return fmt.Errorf("create dependency versions root: %w", err)
	}

	imageID, err := r.imageID(ctx)
	if err != nil {
		return err
	}
	requirementsHash := sha256.Sum256(requirements)
	requirementsHashText := hex.EncodeToString(requirementsHash[:])
	fingerprint := dependencyFingerprint(requirementsHashText, imageID, r.config.PipMirrorURL)
	versionDir := filepath.Join(versionsRoot, fingerprint)
	if manifestMatches(versionDir, fingerprint) {
		if err := activateDependencyVersion(r.config.DependencyRoot, fingerprint); err != nil {
			return err
		}
		r.cleanupDependencyVersions(ctx, fingerprint)
		return nil
	}

	stagingDir, err := os.MkdirTemp(versionsRoot, ".staging-")
	if err != nil {
		return fmt.Errorf("create dependency staging directory: %w", err)
	}
	defer os.RemoveAll(stagingDir)
	if err = os.Chmod(stagingDir, 0o770); err != nil {
		return fmt.Errorf("set dependency staging permissions: %w", err)
	}
	if os.Getuid() == 0 {
		if err = os.Chown(stagingDir, sandboxUserID, sandboxGroupID); err != nil {
			return fmt.Errorf("set dependency staging owner: %w", err)
		}
	}

	buildCtx, cancel := context.WithTimeout(ctx, dependencyBuildTimeout)
	defer cancel()
	containerName := "coze-skill-deps-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	defer r.removeContainer(containerName)

	stdout := newLimitedBuffer(1<<20, cancel)
	stderr := newLimitedBuffer(1<<20, cancel)
	args := r.dependencyBuildArgs(containerName, stagingDir)
	cmd := exec.CommandContext(buildCtx, r.config.DockerBinary, args...)
	cmd.Stdin = bytes.NewReader(requirements)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if runErr := cmd.Run(); runErr != nil {
		if errors.Is(buildCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("dependency build timed out after %s", dependencyBuildTimeout)
		}
		return fmt.Errorf("build Python dependencies: %w: stderr: %s", runErr, strings.TrimSpace(stderr.String()))
	}
	if stdout.exceeded || stderr.exceeded {
		return fmt.Errorf("dependency build output exceeded 1 MB")
	}

	manifest := dependencyManifest{
		Fingerprint:      fingerprint,
		RequirementsHash: requirementsHashText,
		RuntimeImage:     imageID,
		PipMirror:        strings.TrimSpace(r.config.PipMirrorURL),
		CreatedAt:        time.Now().UTC(),
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dependency manifest: %w", err)
	}
	if err = os.WriteFile(filepath.Join(stagingDir, "manifest.json"), manifestBytes, 0o640); err != nil {
		return fmt.Errorf("write dependency manifest: %w", err)
	}
	if err = os.WriteFile(filepath.Join(stagingDir, ".complete"), []byte(fingerprint+"\n"), 0o640); err != nil {
		return fmt.Errorf("write dependency completion marker: %w", err)
	}
	if err = os.Rename(stagingDir, versionDir); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("publish dependency version: %w", err)
		}
	}
	if err = activateDependencyVersion(r.config.DependencyRoot, fingerprint); err != nil {
		return err
	}
	r.cleanupDependencyVersions(ctx, fingerprint)
	return nil
}

// cleanupDependencyVersions bounds local storage while keeping the active
// version and the most recent versions needed for a quick rollback. It is best
// effort so a stale directory cannot make a valid environment unusable.
func (r *Runner) cleanupDependencyVersions(ctx context.Context, activeFingerprint string) {
	keep := r.config.DependencyVersionsToKeep
	if keep < 1 {
		keep = 1
	}
	versionsRoot := filepath.Join(r.config.DependencyRoot, "versions")
	entries, err := os.ReadDir(versionsRoot)
	if err != nil {
		return
	}
	type versionEntry struct {
		name    string
		modTime time.Time
	}
	versions := make([]versionEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".staging-") {
			if removeErr := os.RemoveAll(filepath.Join(versionsRoot, entry.Name())); removeErr != nil {
				logs.CtxWarnf(ctx, "remove stale Skill dependency staging directory failed, path=%s, err=%v", entry.Name(), removeErr)
			}
			continue
		}
		if !manifestMatches(filepath.Join(versionsRoot, entry.Name()), entry.Name()) {
			if removeErr := os.RemoveAll(filepath.Join(versionsRoot, entry.Name())); removeErr != nil {
				logs.CtxWarnf(ctx, "remove incomplete Skill dependency version failed, version=%s, err=%v", entry.Name(), removeErr)
			}
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		versions = append(versions, versionEntry{name: entry.Name(), modTime: info.ModTime()})
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i].modTime.After(versions[j].modTime) })
	keepNames := map[string]struct{}{activeFingerprint: {}}
	for _, version := range versions {
		if len(keepNames) >= keep {
			break
		}
		keepNames[version.name] = struct{}{}
	}
	for _, version := range versions {
		if _, ok := keepNames[version.name]; ok {
			continue
		}
		if removeErr := os.RemoveAll(filepath.Join(versionsRoot, version.name)); removeErr != nil {
			logs.CtxWarnf(ctx, "remove stale Skill dependency version failed, version=%s, err=%v", version.name, removeErr)
		}
	}
}

func (r *Runner) dependencyBuildArgs(containerName, stagingDir string) []string {
	args := []string{
		"run", "--rm", "--interactive",
		"--name", containerName,
		"--pull", "never",
		"--network", r.buildNetwork(),
		"--read-only",
		"--tmpfs", "/tmp:rw,nosuid,nodev,size=512m",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--pids-limit", fmt.Sprintf("%d", dependencyBuildPids),
		"--memory", fmt.Sprintf("%dm", dependencyBuildMemoryMB),
		"--cpus", fmt.Sprintf("%g", dependencyBuildCPUs),
		"--user", r.executionUser(),
		"--env", "HOME=/tmp",
		"--env", "PIP_DISABLE_PIP_VERSION_CHECK=1",
		"--env", "PIP_NO_CACHE_DIR=1",
		"--volume", stagingDir + ":" + containerDependencyRoot + ":rw",
	}
	if mirror := strings.TrimSpace(r.config.PipMirrorURL); mirror != "" {
		args = append(args, "--env", "PIP_INDEX_URL="+mirror)
		if parsed, err := url.Parse(mirror); err == nil && parsed.Scheme == "http" && parsed.Hostname() != "" {
			args = append(args, "--env", "PIP_TRUSTED_HOST="+parsed.Hostname())
		}
	}
	// The output directory is mounted at /opt/skill-deps so wheel metadata and
	// package files are created in the same path used by the execution image.
	args = append(args,
		r.config.DockerImage,
		"python", "-m", "pip", "install", "--no-cache-dir", "--report", filepath.Join(containerDependencyRoot, "pip-report.json"), "--target", containerDependencyRoot, "-r", "/dev/stdin",
	)
	return args
}

func (r *Runner) buildNetwork() string {
	if network := strings.TrimSpace(r.config.BuildNetwork); network != "" {
		return network
	}
	return "bridge"
}

func (r *Runner) imageID(ctx context.Context) (string, error) {
	inspectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(inspectCtx, r.config.DockerBinary, "image", "inspect", "--format", "{{.Id}}", r.config.DockerImage).Output()
	if err != nil {
		return "", fmt.Errorf("inspect dependency runtime image %q: %w", r.config.DockerImage, err)
	}
	imageID := strings.TrimSpace(string(bytes.TrimSpace(output)))
	if imageID == "" {
		return "", fmt.Errorf("dependency runtime image %q has no image ID", r.config.DockerImage)
	}
	return imageID, nil
}

func dependencyFingerprint(requirementsHash, imageID, pipMirror string) string {
	hash := sha256.Sum256([]byte(requirementsHash + "\x00" + imageID + "\x00" + strings.TrimSpace(pipMirror)))
	return hex.EncodeToString(hash[:])
}

func manifestMatches(versionDir, fingerprint string) bool {
	marker, err := os.ReadFile(filepath.Join(versionDir, ".complete"))
	return err == nil && strings.TrimSpace(string(marker)) == fingerprint
}

func activateDependencyVersion(root, fingerprint string) error {
	current := filepath.Join(root, "current")
	temporary := filepath.Join(root, ".current-"+fingerprint)
	_ = os.Remove(temporary)
	if err := os.Symlink(filepath.Join("versions", fingerprint), temporary); err != nil {
		return fmt.Errorf("create current dependency link: %w", err)
	}
	if err := os.Rename(temporary, current); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("activate current dependency version: %w", err)
	}
	return nil
}
