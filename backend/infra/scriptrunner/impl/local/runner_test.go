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
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-studio/backend/infra/scriptrunner"
)

func TestRunnerRun(t *testing.T) {
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not installed")
	}

	root := t.TempDir()
	workspaceRoot := t.TempDir()
	scriptPath := filepath.Join(root, "demo", "echo.py")
	require.NoError(t, os.MkdirAll(filepath.Dir(scriptPath), 0o750))
	require.NoError(t, os.WriteFile(scriptPath, []byte(`import json
import os
import sys

params = json.loads(sys.argv[1])
print(json.dumps({"message": params["message"], "cwd": os.getcwd()}))
`), 0o640))

	runner, err := NewRunner(scriptrunner.Config{
		SkillsRoot:     root,
		WorkspaceRoot:  workspaceRoot,
		PythonPath:     pythonPath,
		TimeoutSeconds: 5,
	})
	require.NoError(t, err)

	response, err := runner.Run(context.Background(), &scriptrunner.RunRequest{
		ScriptPath:     "demo/echo.py",
		ConversationID: 42,
		Params:         map[string]any{"message": "hello"},
	})
	require.NoError(t, err)
	require.Equal(t, 0, response.ExitCode)
	require.Equal(t, filepath.Join(workspaceRoot, "42"), response.WorkingDir)

	var result map[string]string
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(response.Stdout)), &result))
	require.Equal(t, "hello", result["message"])
	realWorkingDir, err := filepath.EvalSymlinks(response.WorkingDir)
	require.NoError(t, err)
	require.Equal(t, realWorkingDir, result["cwd"])
	_, err = os.Stat(response.WorkingDir)
	require.NoError(t, err)

	secondResponse, err := runner.Run(context.Background(), &scriptrunner.RunRequest{
		ScriptPath:     "demo/echo.py",
		ConversationID: 42,
		Params:         map[string]any{"message": "again"},
	})
	require.NoError(t, err)
	require.Equal(t, response.WorkingDir, secondResponse.WorkingDir)

	thirdResponse, err := runner.Run(context.Background(), &scriptrunner.RunRequest{
		ScriptPath:     "demo/echo.py",
		ConversationID: 43,
		Params:         map[string]any{"message": "separate"},
	})
	require.NoError(t, err)
	require.NotEqual(t, response.WorkingDir, thirdResponse.WorkingDir)
}

func TestRunnerRejectsPathOutsideSkillsRoot(t *testing.T) {
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not installed")
	}

	root := t.TempDir()
	runner, err := NewRunner(scriptrunner.Config{
		SkillsRoot:    root,
		WorkspaceRoot: t.TempDir(),
		PythonPath:    pythonPath,
	})
	require.NoError(t, err)

	_, err = runner.Run(context.Background(), &scriptrunner.RunRequest{
		ScriptPath:     "../outside.py",
		ConversationID: 1,
	})
	require.ErrorContains(t, err, "escapes the skills root")
}
