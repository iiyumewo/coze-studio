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
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/coze-dev/coze-studio/backend/infra/scriptrunner"
	runnerimpl "github.com/coze-dev/coze-studio/backend/infra/scriptrunner/impl"
	"github.com/coze-dev/coze-studio/backend/infra/skill"
)

type skillToolConfig struct {
	catalog        *skill.Catalog
	runner         scriptrunner.Runner
	conversationID int64
}

func newSkillTools(_ context.Context, conf *skillToolConfig) ([]tool.InvokableTool, error) {
	if conf == nil || conf.catalog == nil || len(conf.catalog.Skills()) == 0 {
		return []tool.InvokableTool{}, nil
	}

	runner := conf.runner
	if runner == nil {
		var err error
		runner, err = runnerimpl.NewFromEnv(conf.catalog.Root())
		if err != nil {
			return nil, fmt.Errorf("create skill script runner: %w", err)
		}
	}

	readTool, err := utils.InferTool("skill_read", skillReadToolDescription, (&skillReadTool{
		catalog: conf.catalog,
	}).Invoke)
	if err != nil {
		return nil, fmt.Errorf("create skill_read tool: %w", err)
	}

	fileReadTool, err := utils.InferTool("skill_file_read", skillFileReadToolDescription, (&skillFileReadTool{
		catalog: conf.catalog,
	}).Invoke)
	if err != nil {
		return nil, fmt.Errorf("create skill_file_read tool: %w", err)
	}

	scriptTool, err := utils.InferTool("script_run", scriptRunToolDescription, (&scriptRunTool{
		catalog:        conf.catalog,
		runner:         runner,
		conversationID: conf.conversationID,
	}).Invoke)
	if err != nil {
		return nil, fmt.Errorf("create script_run tool: %w", err)
	}

	return []tool.InvokableTool{readTool, fileReadTool, scriptTool}, nil
}

const skillReadToolDescription = `Read the complete instructions of a local Skill.
Call this before using a Skill so you know its available files, scripts, and expected parameters. Use skill_file_read to read referenced files.`

type skillReadInput struct {
	Skill string `json:"skill" jsonschema:"required,description=The Skill id from the available Skills list"`
}

type skillReadTool struct {
	catalog *skill.Catalog
}

func (t *skillReadTool) Invoke(_ context.Context, input *skillReadInput) (string, error) {
	if input == nil || strings.TrimSpace(input.Skill) == "" {
		return "Skill id is required", nil
	}
	definition, ok := t.catalog.Find(input.Skill)
	if !ok {
		return fmt.Sprintf("Skill %q was not found", input.Skill), nil
	}
	instructions, err := t.catalog.ReadInstructions(input.Skill)
	if err != nil {
		return fmt.Sprintf("Failed to read Skill %q instructions: %v", input.Skill, err), nil
	}
	return fmt.Sprintf("Skill %s (id: %s) instructions:\n%s", definition.Name, definition.ID, instructions), nil
}

const skillFileReadToolDescription = `Read a text file from a local Skill.
The path is relative to the selected Skill directory. Use this for referenced Markdown, YAML, JSON, or text files. Do not use script_run to read files.`

type skillFileReadInput struct {
	Skill string `json:"skill" jsonschema:"required,description=The Skill id from the available Skills list"`
	Path  string `json:"path" jsonschema:"required,description=The file path relative to the selected Skill directory"`
}

type skillFileReadTool struct {
	catalog *skill.Catalog
}

func (t *skillFileReadTool) Invoke(_ context.Context, input *skillFileReadInput) (string, error) {
	if input == nil || strings.TrimSpace(input.Skill) == "" {
		return "Skill id is required", nil
	}
	if strings.TrimSpace(input.Path) == "" {
		return "File path is required", nil
	}
	content, err := t.catalog.ReadFile(input.Skill, input.Path)
	if err != nil {
		return fmt.Sprintf("Failed to read Skill file: %v", err), nil
	}
	return fmt.Sprintf("Skill file %s (%s):\n%s", input.Skill, input.Path, content), nil
}

const scriptRunToolDescription = `Run a Python script from a local Skill.
The script_path must be a .py file relative to the selected Skill directory. Pass script parameters in params as a JSON object.
Read the Skill instructions with skill_read first, then use skill_file_read for referenced files. The script runs in the current conversation's working directory, which can be reused by later calls.`

type scriptRunInput struct {
	Skill      string         `json:"skill" jsonschema:"required,description=The Skill id from the available Skills list"`
	ScriptPath string         `json:"script_path" jsonschema:"required,description=The Python script path relative to the Skill directory"`
	Params     map[string]any `json:"params" jsonschema:"description=JSON object passed to the script"`
}

type scriptRunTool struct {
	catalog        *skill.Catalog
	runner         scriptrunner.Runner
	conversationID int64
}

func (t *scriptRunTool) Invoke(ctx context.Context, input *scriptRunInput) (string, error) {
	if input == nil {
		return "Script run input is required", nil
	}
	scriptPath, err := t.catalog.ResolveScriptPath(input.Skill, input.ScriptPath)
	if err != nil {
		return fmt.Sprintf("Script was not run: %v", err), nil
	}
	response, err := t.runner.Run(ctx, &scriptrunner.RunRequest{
		ScriptPath:     scriptPath,
		ConversationID: t.conversationID,
		Params:         input.Params,
	})
	if response == nil {
		if err != nil {
			return fmt.Sprintf("Script execution failed: %v", err), nil
		}
		return "Script execution returned no response", nil
	}

	var result strings.Builder
	if err != nil {
		fmt.Fprintf(&result, "Script execution failed: %v\n", err)
	}
	fmt.Fprintf(&result, "exit_code: %d\nworking_directory: %s", response.ExitCode, response.WorkingDir)
	if strings.TrimSpace(response.Stdout) != "" {
		fmt.Fprintf(&result, "\nstdout:\n%s", response.Stdout)
	}
	if strings.TrimSpace(response.Stderr) != "" {
		fmt.Fprintf(&result, "\nstderr:\n%s", response.Stderr)
	}
	return result.String(), nil
}
