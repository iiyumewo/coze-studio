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

package agentflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-studio/backend/infra/scriptrunner"
	localrunner "github.com/coze-dev/coze-studio/backend/infra/scriptrunner/impl/local"
	"github.com/coze-dev/coze-studio/backend/infra/skill"
)

func TestSkillTools(t *testing.T) {
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not installed")
	}

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "calculator"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "calculator", "SKILL.md"), []byte(`---
name: Calculator
description: Add two numbers.
---

Use calculator.py with parameters a and b.
`), 0o640))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "calculator", "references"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "calculator", "references", "rules.md"), []byte("# Rules\nUse the reference.\n"), 0o640))
	require.NoError(t, os.WriteFile(filepath.Join(root, "calculator", "calculator.py"), []byte(`import json
import sys

params = json.loads(sys.argv[1])
print(json.dumps({"sum": params["a"] + params["b"]}))
`), 0o640))

	catalog, err := skill.LoadCatalog(root)
	require.NoError(t, err)
	runner, err := localrunner.NewRunner(scriptrunner.Config{
		SkillsRoot:    root,
		WorkspaceRoot: t.TempDir(),
		PythonPath:    pythonPath,
	})
	require.NoError(t, err)

	tools, err := newSkillTools(context.Background(), &skillToolConfig{
		catalog:        catalog,
		runner:         runner,
		conversationID: 100,
	})
	require.NoError(t, err)
	require.Len(t, tools, 3)

	readInfo, err := tools[0].Info(context.Background())
	require.NoError(t, err)
	require.Equal(t, "skill_read", readInfo.Name)
	readOutput, err := tools[0].InvokableRun(context.Background(), `{"skill":"calculator"}`)
	require.NoError(t, err)
	require.Contains(t, readOutput, "calculator.py")

	fileReadInfo, err := tools[1].Info(context.Background())
	require.NoError(t, err)
	require.Equal(t, "skill_file_read", fileReadInfo.Name)
	fileOutput, err := tools[1].InvokableRun(context.Background(), `{"skill":"calculator","path":"references/rules.md"}`)
	require.NoError(t, err)
	require.Contains(t, fileOutput, "Use the reference")

	runInfo, err := tools[2].Info(context.Background())
	require.NoError(t, err)
	require.Equal(t, "script_run", runInfo.Name)
	runOutput, err := tools[2].InvokableRun(context.Background(), `{"skill":"calculator","script_path":"calculator.py","params":{"a":2,"b":3}}`)
	require.NoError(t, err)
	require.Contains(t, runOutput, `"sum": 5`)
	require.Contains(t, runOutput, "working_directory")

	fileRunOutput, err := tools[2].InvokableRun(context.Background(), `{"skill":"calculator","script_path":"references/rules.md","params":{}}`)
	require.NoError(t, err)
	require.Contains(t, fileRunOutput, "must have a .py extension")
}

func TestSkillToolsReturnsExecutionFailureToModel(t *testing.T) {
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not installed")
	}

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "broken"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "broken", "SKILL.md"), []byte("---\nname: Broken\ndescription: Fails.\n---\n"), 0o640))
	require.NoError(t, os.WriteFile(filepath.Join(root, "broken", "broken.py"), []byte("raise RuntimeError('expected failure')\n"), 0o640))

	catalog, err := skill.LoadCatalog(root)
	require.NoError(t, err)
	runner, err := localrunner.NewRunner(scriptrunner.Config{
		SkillsRoot:    root,
		WorkspaceRoot: t.TempDir(),
		PythonPath:    pythonPath,
	})
	require.NoError(t, err)

	tools, err := newSkillTools(context.Background(), &skillToolConfig{
		catalog:        catalog,
		runner:         runner,
		conversationID: 101,
	})
	require.NoError(t, err)
	output, err := tools[2].InvokableRun(context.Background(), `{"skill":"broken","script_path":"broken.py","params":{}}`)
	require.NoError(t, err)
	require.Contains(t, output, "Script execution failed")
	require.True(t, strings.Contains(output, "expected failure"))
}
