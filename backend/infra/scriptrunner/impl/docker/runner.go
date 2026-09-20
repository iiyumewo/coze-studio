/*
 * Copyright 2025 coze-dev Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/coze-dev/coze-studio/backend/infra/scriptrunner"
	"github.com/coze-dev/coze-studio/backend/infra/skill"
)

const (
	defaultImage         = "python:3.12-slim-bookworm"
	defaultTimeout       = 60 * time.Second
	defaultMaxOutputSize = 1 << 20
	defaultMemoryLimitMB = 256
	defaultCPUs          = 0.5
	defaultPidsLimit     = 64
	sandboxUserID        = 65532
	sandboxGroupID       = 65532
	containerSkillsRoot  = "/opt/skill"
	containerWorkspace   = "/workspace"
)

var errOutputLimitExceeded = errors.New("script output limit exceeded")

func NewRunner(config scriptrunner.Config) (*Runner, error) {
	if config.SkillsRoot == "" {
		config.SkillsRoot = skill.DefaultRoot()
	}
	if config.WorkspaceRoot == "" {
		config.WorkspaceRoot = skill.DefaultWorkspaceRoot()
	}
	if config.DependencyRoot == "" {
		config.DependencyRoot = skill.DefaultDependencyRoot()
	}

	var err error
	if config.SkillsRoot, err = filepath.Abs(config.SkillsRoot); err != nil {
		return nil, fmt.Errorf("resolve skills root: %w", err)
	}
	if config.WorkspaceRoot, err = filepath.Abs(config.WorkspaceRoot); err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	if config.DependencyRoot, err = filepath.Abs(config.DependencyRoot); err != nil {
		return nil, fmt.Errorf("resolve dependency root: %w", err)
	}
	if strings.TrimSpace(config.RequirementsPath) != "" {
		if config.RequirementsPath, err = filepath.Abs(config.RequirementsPath); err != nil {
			return nil, fmt.Errorf("resolve requirements path: %w", err)
		}
	}
	if config.DockerBinary == "" {
		config.DockerBinary = "docker"
	}
	if config.DockerBinary, err = exec.LookPath(config.DockerBinary); err != nil {
		return nil, fmt.Errorf("find docker binary: %w", err)
	}
	if config.DockerImage == "" {
		config.DockerImage = defaultImage
	}
	if config.TimeoutSeconds <= 0 {
		config.TimeoutSeconds = int(defaultTimeout / time.Second)
	}
	if config.MaxOutputBytes <= 0 {
		config.MaxOutputBytes = defaultMaxOutputSize
	}
	if config.DockerMemoryLimitMB <= 0 {
		config.DockerMemoryLimitMB = defaultMemoryLimitMB
	}
	if config.DockerCPUs <= 0 {
		config.DockerCPUs = defaultCPUs
	}
	if config.DockerPidsLimit <= 0 {
		config.DockerPidsLimit = defaultPidsLimit
	}

	return &Runner{config: config}, nil
}

type Runner struct {
	config scriptrunner.Config
}

func (r *Runner) Run(ctx context.Context, request *scriptrunner.RunRequest) (*scriptrunner.RunResponse, error) {
	if request == nil {
		return nil, errors.New("script run request is nil")
	}
	if request.ConversationID <= 0 {
		return nil, errors.New("conversation ID must be positive")
	}

	scriptPath, skillDir, containerScriptPath, err := r.resolveScript(request.ScriptPath)
	if err != nil {
		return nil, err
	}
	if err = validateScriptFile(r.config.SkillsRoot, scriptPath); err != nil {
		return nil, err
	}

	workingDir := filepath.Join(r.config.WorkspaceRoot, strconv.FormatInt(request.ConversationID, 10))
	if err = os.MkdirAll(filepath.Join(workingDir, ".tmp"), 0o750); err != nil {
		return nil, fmt.Errorf("create conversation workspace: %w", err)
	}
	if os.Getuid() == 0 {
		if err = os.Chown(workingDir, sandboxUserID, sandboxGroupID); err != nil {
			return nil, fmt.Errorf("set conversation workspace owner: %w", err)
		}
		if err = os.Chown(filepath.Join(workingDir, ".tmp"), sandboxUserID, sandboxGroupID); err != nil {
			return nil, fmt.Errorf("set conversation temp directory owner: %w", err)
		}
	}
	workingDir, err = filepath.EvalSymlinks(workingDir)
	if err != nil {
		return nil, fmt.Errorf("resolve conversation workspace: %w", err)
	}
	if err = validateDirectoryWithinRoot(r.config.WorkspaceRoot, workingDir); err != nil {
		return nil, fmt.Errorf("conversation workspace is invalid: %w", err)
	}
	dependencyDir := r.currentDependencyDir()
	skillDir, err = filepath.EvalSymlinks(skillDir)
	if err != nil {
		return nil, fmt.Errorf("resolve skill directory: %w", err)
	}
	if err = validateDirectoryWithinRoot(r.config.SkillsRoot, skillDir); err != nil {
		return nil, fmt.Errorf("skill directory is invalid: %w", err)
	}

	params := request.Params
	if params == nil {
		params = map[string]any{}
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal script params: %w", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(r.config.TimeoutSeconds)*time.Second)
	defer cancel()

	containerName := "coze-skill-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	defer r.removeContainer(containerName)

	stdout := newLimitedBuffer(r.config.MaxOutputBytes, cancel)
	stderr := newLimitedBuffer(r.config.MaxOutputBytes, cancel)
	cmd := exec.CommandContext(runCtx, r.config.DockerBinary, r.dockerArgs(
		containerName,
		skillDir,
		workingDir,
		dependencyDir,
		containerScriptPath,
		string(paramsJSON),
	)...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	runErr := cmd.Run()
	response := &scriptrunner.RunResponse{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitCode:   exitCode(cmd),
		WorkingDir: workingDir,
	}
	if stdout.exceeded || stderr.exceeded {
		return response, fmt.Errorf("script output exceeded %d bytes", r.config.MaxOutputBytes)
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return response, fmt.Errorf("script execution timed out after %d seconds", r.config.TimeoutSeconds)
	}
	if runErr != nil {
		return response, fmt.Errorf("run script in docker: %w: stderr: %s", runErr, strings.TrimSpace(response.Stderr))
	}
	return response, nil
}

func (r *Runner) resolveScript(scriptPath string) (hostScriptPath, skillDir, containerScriptPath string, err error) {
	if strings.TrimSpace(scriptPath) == "" {
		return "", "", "", errors.New("script path is required")
	}
	if filepath.IsAbs(scriptPath) {
		return "", "", "", errors.New("script path must be relative to the skills root")
	}
	cleanPath := filepath.Clean(scriptPath)
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(os.PathSeparator)) {
		return "", "", "", errors.New("script path escapes the skills root")
	}
	pathParts := strings.Split(cleanPath, string(os.PathSeparator))
	if len(pathParts) < 2 {
		return "", "", "", errors.New("script path must include the skill directory")
	}

	skillDir = filepath.Join(r.config.SkillsRoot, pathParts[0])
	hostScriptPath = filepath.Join(r.config.SkillsRoot, cleanPath)
	relativeScriptPath, err := filepath.Rel(skillDir, hostScriptPath)
	if err != nil || relativeScriptPath == ".." || strings.HasPrefix(relativeScriptPath, ".."+string(os.PathSeparator)) {
		return "", "", "", errors.New("script path escapes the skill directory")
	}
	containerScriptPath = containerSkillsRoot + "/" + filepath.ToSlash(relativeScriptPath)
	return hostScriptPath, skillDir, containerScriptPath, nil
}

func (r *Runner) dockerArgs(containerName, skillDir, workingDir, dependencyDir, scriptPath, paramsJSON string) []string {
	args := []string{
		"run",
		"--rm",
		"--name", containerName,
		"--pull", "never",
		"--network", "none",
		"--read-only",
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=64m",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--pids-limit", strconv.FormatInt(r.config.DockerPidsLimit, 10),
		"--memory", fmt.Sprintf("%dm", r.config.DockerMemoryLimitMB),
		"--cpus", strconv.FormatFloat(r.config.DockerCPUs, 'f', -1, 64),
		"--user", r.executionUser(),
		"--env", "PYTHONDONTWRITEBYTECODE=1",
		"--env", "PYTHONUNBUFFERED=1",
		"--env", "HOME=" + containerWorkspace,
		"--env", "TMPDIR=" + containerWorkspace + "/.tmp",
		"--volume", skillDir + ":" + containerSkillsRoot + ":ro",
		"--volume", workingDir + ":" + containerWorkspace + ":rw",
		"--workdir", containerWorkspace,
	}
	if dependencyDir != "" {
		args = append(args,
			"--env", "PYTHONPATH="+containerDependencyRoot,
			"--volume", dependencyDir+":"+containerDependencyRoot+":ro",
		)
	}
	args = append(args, r.config.DockerImage, "python", scriptPath, paramsJSON)
	return args
}

func (r *Runner) currentDependencyDir() string {
	current := filepath.Join(r.config.DependencyRoot, "current")
	info, err := os.Stat(current)
	if err != nil || !info.IsDir() {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(current)
	if err != nil || validateDirectoryWithinRoot(r.config.DependencyRoot, resolved) != nil {
		return ""
	}
	return resolved
}

func (r *Runner) executionUser() string {
	uid, gid := os.Getuid(), os.Getgid()
	if uid == 0 {
		return fmt.Sprintf("%d:%d", sandboxUserID, sandboxGroupID)
	}
	return fmt.Sprintf("%d:%d", uid, gid)
}

func (r *Runner) removeContainer(containerName string) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = exec.CommandContext(cleanupCtx, r.config.DockerBinary, "rm", "-f", containerName).Run()
}

func validateScriptFile(root, scriptPath string) error {
	info, err := os.Stat(scriptPath)
	if err != nil {
		return fmt.Errorf("stat script: %w", err)
	}
	if info.IsDir() {
		return errors.New("script path points to a directory")
	}

	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve skills root: %w", err)
	}
	scriptReal, err := filepath.EvalSymlinks(scriptPath)
	if err != nil {
		return fmt.Errorf("resolve script path: %w", err)
	}
	rel, err := filepath.Rel(rootReal, scriptReal)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return errors.New("script path resolves outside the skills root")
	}
	return nil
}

func validateDirectoryWithinRoot(root, directory string) error {
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}
	directoryReal, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return fmt.Errorf("resolve directory: %w", err)
	}
	rel, err := filepath.Rel(rootReal, directoryReal)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return errors.New("path resolves outside the configured root")
	}
	return nil
}

func exitCode(cmd *exec.Cmd) int {
	if cmd.ProcessState == nil {
		return -1
	}
	return cmd.ProcessState.ExitCode()
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	cancel   context.CancelFunc
	exceeded bool
}

func newLimitedBuffer(limit int, cancel context.CancelFunc) *limitedBuffer {
	return &limitedBuffer{limit: limit, cancel: cancel}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.exceeded {
		return 0, errOutputLimitExceeded
	}
	remaining := b.limit - b.Len()
	if len(p) > remaining {
		if remaining > 0 {
			_, _ = b.Buffer.Write(p[:remaining])
		}
		b.exceeded = true
		b.cancel()
		return len(p), errOutputLimitExceeded
	}
	return b.Buffer.Write(p)
}
