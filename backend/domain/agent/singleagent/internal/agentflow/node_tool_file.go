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

	"github.com/coze-dev/coze-studio/backend/domain/agent/singleagent/entity"
)

type fileSearchToolConfig struct {
	ConversationID int64
	AgentID        int64
	UserID         string
	Search         entity.FileSearchFunc
	Download       entity.FileDownloadFunc
	Upload         entity.FileUploadFunc
}

type fileSearchInput struct {
	FileRef string `json:"file_ref" jsonschema:"description=Optional file name or reference. Leave empty to search all files in the current conversation."`
	Query   string `json:"query" jsonschema:"required,description=The question or information to find in the conversation files"`
}

func newFileSearchTools(_ context.Context, conf *fileSearchToolConfig) ([]tool.InvokableTool, error) {
	if conf == nil || (conf.Search == nil && conf.Download == nil && conf.Upload == nil) {
		return []tool.InvokableTool{}, nil
	}

	tools := make([]tool.InvokableTool, 0, 3)
	if conf.Search != nil {
		searchTool, err := utils.InferTool(
			"file_search",
			"Search files attached to the current conversation. Use this tool when the user asks about a historical or uploaded file. The backend will read the file from its own storage, use the parse cache when available, and parse it on a cache miss. Do not invent file URLs.",
			func(ctx context.Context, input *fileSearchInput) (string, error) {
				return conf.Search(ctx, conf.ConversationID, conf.AgentID, conf.UserID, input.FileRef, input.Query)
			},
		)
		if err != nil {
			return nil, err
		}
		tools = append(tools, searchTool)
	}

	if conf.Download != nil {
		downloadTool, err := utils.InferTool(
			"file_download",
			"Download an uploaded file from the current conversation into the current Skill workspace. Use this when a Skill script needs the original binary file, such as an Excel or Word document. Prefer file_key: pass the exact URI/file_key from the attachment metadata, because filenames are not unique. Use file_name only as a compatibility fallback when it identifies exactly one file. If multiple files have the same name, do not guess. The returned path is relative to the script workspace and can be passed directly to script_run. Do not invent file paths or URLs.",
			func(ctx context.Context, input *fileDownloadInput) (string, error) {
				if input == nil {
					return "file_key or file_name is required", nil
				}
				fileRef := strings.TrimSpace(input.FileKey)
				if fileRef == "" {
					fileRef = strings.TrimSpace(input.FileName)
				}
				if fileRef == "" {
					return "file_key or file_name is required", nil
				}
				return conf.Download(ctx, conf.ConversationID, conf.AgentID, conf.UserID, fileRef)
			},
		)
		if err != nil {
			return nil, err
		}
		tools = append(tools, downloadTool)
	}

	if conf.Upload != nil {
		uploadTool, err := utils.InferTool(
			"file_upload",
			"Upload a selected file from the current Skill workspace for the user to download. Use this only for the final files the user should receive, not intermediate files or caches. The path must be returned by script_run or be a relative path in the current workspace. The backend validates the path and returns the uploaded file metadata. The platform will attach a downloadable file card to the response. Never expose or invent a file:// URL, object-storage URL, local path, or Markdown link for the uploaded file.",
			func(ctx context.Context, input *fileUploadInput) (string, error) {
				if input == nil || strings.TrimSpace(input.Path) == "" {
					return "path is required", nil
				}
				file, err := conf.Upload(ctx, conf.ConversationID, conf.AgentID, conf.UserID, input.Path)
				if err != nil {
					return "file upload failed: " + err.Error(), nil
				}
				if collector := entity.UploadedFileCollectorFromContext(ctx); collector != nil {
					collector.Add(file)
				}
				return fmt.Sprintf("uploaded file for platform delivery: file_name=%s, file_type=%s, file_size=%d. A downloadable file card will be attached by the platform; do not create a URL or Markdown link.", file.Name, file.Type, file.Size), nil
			},
		)
		if err != nil {
			return nil, err
		}
		tools = append(tools, uploadTool)
	}

	return tools, nil
}

type fileDownloadInput struct {
	FileKey  string `json:"file_key,omitempty" jsonschema:"description=Exact URI or file_key from the current conversation attachment metadata; preferred because filenames are not unique"`
	FileName string `json:"file_name,omitempty" jsonschema:"description=Original uploaded filename; compatibility fallback only when it is unique"`
}

type fileUploadInput struct {
	Path string `json:"path" jsonschema:"required,description=Workspace-relative path of the selected final file to upload"`
}
