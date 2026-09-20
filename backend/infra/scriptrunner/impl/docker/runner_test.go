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

func TestRunnerDockerArgsUseSandboxRestrictions(t *testing.T) {
	runner := &Runner{config: scriptrunner.Config{
		DockerImage:         defaultImage,
		DockerMemoryLimitMB: 256,
		DockerCPUs:          0.5,
		DockerPidsLimit:     64,
	}}
	args := runner.dockerArgs("sandbox", "/skills/demo", "/work/42", "", "/opt/skill/main.py", `{}`)
	joined := strings.Join(args, " ")
	require.Contains(t, joined, "--network none")
	require.Contains(t, joined, "--pull never")
	require.Contains(t, joined, "--read-only")
	require.Contains(t, joined, "--tmpfs /tmp:rw,noexec,nosuid,nodev,size=64m")
	require.Contains(t, joined, "--cap-drop ALL")
	require.Contains(t, joined, "no-new-privileges")
	require.Contains(t, joined, "/skills/demo:/opt/skill:ro")
	require.Contains(t, joined, "/work/42:/workspace:rw")
	require.NotContains(t, joined, "/var/run/docker.sock")
}

func TestRunnerDockerArgsMountCurrentDependenciesReadOnly(t *testing.T) {
	runner := &Runner{config: scriptrunner.Config{DockerImage: defaultImage}}
	args := runner.dockerArgs("sandbox", "/skills/demo", "/work/42", "/runtime/skill-deps/versions/abc", "/opt/skill/main.py", `{}`)
	joined := strings.Join(args, " ")
	require.Contains(t, joined, "PYTHONPATH=/opt/skill-deps")
	require.Contains(t, joined, "/runtime/skill-deps/versions/abc:/opt/skill-deps:ro")
}

func TestRunnerDockerIntegration(t *testing.T) {
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

	skillsRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	writeTestSkills(t, skillsRoot)
	runner, err := NewRunner(scriptrunner.Config{
		SkillsRoot:          skillsRoot,
		WorkspaceRoot:       workspaceRoot,
		DockerImage:         defaultImage,
		TimeoutSeconds:      20,
		DockerMemoryLimitMB: 128,
		DockerCPUs:          0.5,
		DockerPidsLimit:     32,
	})
	require.NoError(t, err)

	t.Run("multi file import and workspace output", func(t *testing.T) {
		response, runErr := runner.Run(context.Background(), &scriptrunner.RunRequest{
			ScriptPath:     "report/build.py",
			ConversationID: 101,
			Params: map[string]any{
				"title": "Docker report",
				"items": []string{"first", "second"},
			},
		})
		require.NoError(t, runErr)
		require.Equal(t, 0, response.ExitCode)
		require.FileExists(t, filepath.Join(workspaceRoot, "101", "report.txt"))
		content, readErr := os.ReadFile(filepath.Join(workspaceRoot, "101", "report.txt"))
		require.NoError(t, readErr)
		require.Equal(t, "# Docker report\n1. first\n2. second\n", string(content))
	})

	t.Run("workspace persists within a conversation", func(t *testing.T) {
		first := runNotes(t, runner, 201, "alpha")
		require.Equal(t, []string{"alpha"}, first)
		second := runNotes(t, runner, 201, "beta")
		require.Equal(t, []string{"alpha", "beta"}, second)
		separate := runNotes(t, runner, 202, "other")
		require.Equal(t, []string{"other"}, separate)
	})

	t.Run("skill mount is read only", func(t *testing.T) {
		response, runErr := runner.Run(context.Background(), &scriptrunner.RunRequest{
			ScriptPath:     "readonly/write.py",
			ConversationID: 301,
		})
		require.Error(t, runErr)
		require.NotZero(t, response.ExitCode)
		require.Contains(t, response.Stderr, "Read-only file system")
	})

	t.Run("network is disabled", func(t *testing.T) {
		response, runErr := runner.Run(context.Background(), &scriptrunner.RunRequest{
			ScriptPath:     "network/connect.py",
			ConversationID: 401,
		})
		require.Error(t, runErr)
		require.NotZero(t, response.ExitCode)
		require.Contains(t, response.Stderr, "Network is unreachable")
	})
}

func runNotes(t *testing.T, runner *Runner, conversationID int64, note string) []string {
	t.Helper()
	response, err := runner.Run(context.Background(), &scriptrunner.RunRequest{
		ScriptPath:     "notes/notes.py",
		ConversationID: conversationID,
		Params:         map[string]any{"note": note},
	})
	require.NoError(t, err)
	var result struct {
		Notes []string `json:"notes"`
	}
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(response.Stdout)), &result))
	return result.Notes
}

func writeTestSkills(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"report/format.py": `def build(title, items):
    return "# " + title + "\n" + "\n".join(f"{i}. {item}" for i, item in enumerate(items, 1)) + "\n"
`,
		"report/build.py": `import json
import pathlib
import sys
from format import build

params = json.loads(sys.argv[1])
content = build(params["title"], params["items"])
pathlib.Path("report.txt").write_text(content, encoding="utf-8")
print(json.dumps({"content": content}))
`,
		"notes/notes.py": `import json
import pathlib
import sys

params = json.loads(sys.argv[1])
path = pathlib.Path("notes.txt")
with path.open("a", encoding="utf-8") as file:
    file.write(params["note"] + "\n")
print(json.dumps({"notes": path.read_text(encoding="utf-8").splitlines()}))
`,
		"readonly/write.py": `import pathlib
pathlib.Path("/opt/skill/created.txt").write_text("blocked", encoding="utf-8")
`,
		"network/connect.py": `import socket
sock = socket.socket()
sock.settimeout(2)
sock.connect(("1.1.1.1", 80))
`,
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o640))
	}
}

func ensureDockerImage(t *testing.T, image string) {
	t.Helper()
	if err := exec.Command("docker", "image", "inspect", image).Run(); err == nil {
		return
	}
	command := exec.Command("docker", "pull", image)
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "pull Docker image %s: %s", image, output)
}
