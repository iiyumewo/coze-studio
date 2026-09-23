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
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/coze-dev/coze-studio/backend/infra/scriptrunner"
	runnerimpl "github.com/coze-dev/coze-studio/backend/infra/scriptrunner/impl"
	"github.com/coze-dev/coze-studio/backend/infra/skill"
)

const (
	maxSkillFileReadBatchFiles = 16
	maxSkillFileReadBatchBytes = 256 << 10
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
Call this before using a Skill so you know its available files, scripts, and expected parameters. If the user asks which files or reference documents the Skill contains, answer from this result and do not call skill_file_read just to list or verify filenames. Use skill_file_read only when the actual contents of a referenced file are needed.`

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
The path is relative to the selected Skill directory. Use this only when the actual contents of referenced Markdown, YAML, JSON, or text files are needed for the current task. For one file, pass path. For several related reference files, pass paths as an ordered array; this avoids spending one model/tool turn per file. The batch is limited to 16 files and 256 KiB total content. For questions that only ask which files exist, use skill_read instead. Do not use script_run to read files.`

type skillFileReadInput struct {
	Skill string   `json:"skill" jsonschema:"required,description=The Skill id from the available Skills list"`
	Path  string   `json:"path,omitempty" jsonschema:"description=The file path relative to the selected Skill directory; use for one file"`
	Paths []string `json:"paths" jsonschema:"description=Ordered list of file paths relative to the selected Skill directory; use for reading multiple related references in one call"`
}

type skillFileReadTool struct {
	catalog *skill.Catalog
}

func (t *skillFileReadTool) Invoke(_ context.Context, input *skillFileReadInput) (string, error) {
	if input == nil || strings.TrimSpace(input.Skill) == "" {
		return "Skill id is required", nil
	}
	paths := make([]string, 0, len(input.Paths)+1)
	if strings.TrimSpace(input.Path) != "" {
		paths = append(paths, input.Path)
	}
	for _, filePath := range input.Paths {
		if strings.TrimSpace(filePath) != "" {
			paths = append(paths, filePath)
		}
	}
	if len(paths) == 0 {
		return "File path or paths are required", nil
	}
	if len(paths) > maxSkillFileReadBatchFiles {
		return fmt.Sprintf("Skill file batch exceeds %d files", maxSkillFileReadBatchFiles), nil
	}

	var result strings.Builder
	totalBytes := 0
	for _, filePath := range paths {
		content, err := t.catalog.ReadFile(input.Skill, filePath)
		if err != nil {
			return fmt.Sprintf("Failed to read Skill file %s: %v", filePath, err), nil
		}
		totalBytes += len([]byte(content))
		if totalBytes > maxSkillFileReadBatchBytes {
			return fmt.Sprintf("Skill file batch exceeds %d bytes", maxSkillFileReadBatchBytes), nil
		}
		if result.Len() > 0 {
			result.WriteString("\n\n")
		}
		fmt.Fprintf(&result, "Skill file %s (%s):\n%s", input.Skill, filePath, content)
	}
	return result.String(), nil
}

const scriptRunToolDescription = `Run a Python script from a local Skill.
The script_path must be a .py file relative to the selected Skill directory. Read the Skill instructions with skill_read first, then use skill_file_read for referenced Skill files. If the script needs a user-uploaded binary file, call file_download first and pass its returned workspace-relative path to the script.

Argument modes:
- mode="json" (default): pass a JSON object in params. The script receives it as one string in sys.argv[1], which is the existing Skill contract.
- mode="argv": pass an ordered string array in args. The script receives these values unchanged as sys.argv[1:]. Use this for argparse-style scripts, including subcommands. For example, args=["run", "--month", "2026-08", "--outdir", "report"]. Do not put command-line flags in params when using argv mode.

The script runs in the current conversation's working directory, which can be reused by later calls. After execution, workspace_files lists regular files currently available in that workspace; choose only user-facing outputs for file_upload.`

type scriptRunInput struct {
	Skill      string                    `json:"skill" jsonschema:"required,description=The Skill id from the available Skills list"`
	ScriptPath string                    `json:"script_path" jsonschema:"required,description=The Python script path relative to the Skill directory"`
	Mode       scriptrunner.ArgumentMode `json:"mode" jsonschema:"description=Argument mode: json (default) passes params as one JSON argument; argv passes args unchanged as command-line arguments"`
	Params     map[string]any            `json:"params" jsonschema:"description=JSON object passed to the script in json mode"`
	Args       []string                  `json:"args" jsonschema:"description=Ordered command-line arguments passed unchanged in argv mode"`
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
		Mode:           input.Mode,
		Params:         input.Params,
		Args:           input.Args,
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
	if files := listWorkspaceFiles(response.WorkingDir); len(files) > 0 {
		result.WriteString("\nworkspace_files:\n")
		for _, file := range files {
			fmt.Fprintf(&result, "- path: %s\n  size: %d\n  type: %s\n", file.Path, file.Size, file.Type)
		}
	}
	return result.String(), nil
}

type workspaceFile struct {
	Path string
	Size int64
	Type string
}

func listWorkspaceFiles(root string) []workspaceFile {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	files := make([]workspaceFile, 0)
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") || filepath.Ext(entry.Name()) == ".pyc" {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		fileType := mime.TypeByExtension(filepath.Ext(entry.Name()))
		if fileType == "" {
			fileType = "application/octet-stream"
		}
		files = append(files, workspaceFile{Path: filepath.ToSlash(relative), Size: info.Size(), Type: fileType})
		return nil
	})
	return files
}
