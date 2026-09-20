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

package local

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

	"github.com/coze-dev/coze-studio/backend/bizpkg/fileutil"
	"github.com/coze-dev/coze-studio/backend/infra/scriptrunner"
	"github.com/coze-dev/coze-studio/backend/infra/skill"
)

const (
	defaultTimeout       = 60 * time.Second
	defaultMaxOutputSize = 1 << 20
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
	if config.PythonPath == "" {
		config.PythonPath = fileutil.GetPython3Path()
	}
	if config.TimeoutSeconds <= 0 {
		config.TimeoutSeconds = int(defaultTimeout / time.Second)
	}
	if config.MaxOutputBytes <= 0 {
		config.MaxOutputBytes = defaultMaxOutputSize
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

	scriptPath, err := resolveScriptPath(r.config.SkillsRoot, request.ScriptPath)
	if err != nil {
		return nil, err
	}
	if err = validateScriptFile(r.config.SkillsRoot, scriptPath); err != nil {
		return nil, err
	}

	workingDir := filepath.Join(r.config.WorkspaceRoot, strconv.FormatInt(request.ConversationID, 10))
	if err = os.MkdirAll(workingDir, 0o750); err != nil {
		return nil, fmt.Errorf("create conversation workspace: %w", err)
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

	stdout := newLimitedBuffer(r.config.MaxOutputBytes, cancel)
	stderr := newLimitedBuffer(r.config.MaxOutputBytes, cancel)
	cmd := exec.CommandContext(runCtx, r.config.PythonPath, scriptPath, string(paramsJSON)) // ignore_security_alert RCE
	cmd.Dir = workingDir
	cmd.Env = os.Environ()
	if dependencyEnv := r.dependencyEnv(); dependencyEnv != "" {
		cmd.Env = setEnv(cmd.Env, "PYTHONPATH", dependencyEnv)
	}
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
		return response, fmt.Errorf("run script: %w: stderr: %s", runErr, strings.TrimSpace(response.Stderr))
	}
	return response, nil
}

func (r *Runner) dependencyEnv() string {
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

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func validateDirectoryWithinRoot(root, directory string) error {
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	directoryReal, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootReal, directoryReal)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return errors.New("path resolves outside the configured root")
	}
	return nil
}

func resolveScriptPath(root, scriptPath string) (string, error) {
	if strings.TrimSpace(scriptPath) == "" {
		return "", errors.New("script path is required")
	}
	if filepath.IsAbs(scriptPath) {
		return "", errors.New("script path must be relative to the skills root")
	}
	cleanPath := filepath.Clean(scriptPath)
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(os.PathSeparator)) {
		return "", errors.New("script path escapes the skills root")
	}
	return filepath.Join(root, cleanPath), nil
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
