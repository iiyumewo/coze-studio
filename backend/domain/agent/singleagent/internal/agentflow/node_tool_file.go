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
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
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
	FileKey  string `json:"file_key,omitempty" jsonschema:"description=Exact URI or file_key from the current conversation attachment metadata; preferred because filenames are not unique"`
	FileName string `json:"file_name,omitempty" jsonschema:"description=Original uploaded filename; if duplicated, the newest matching file is selected"`
	Query    string `json:"query" jsonschema:"required,description=The question or information to find in the conversation files"`
}

func newFileSearchTools(_ context.Context, conf *fileSearchToolConfig) ([]tool.InvokableTool, error) {
	if conf == nil || (conf.Search == nil && conf.Download == nil && conf.Upload == nil) {
		return []tool.InvokableTool{}, nil
	}

	tools := make([]tool.InvokableTool, 0, 3)
	if conf.Search != nil {
		searchTool, err := utils.InferTool(
			"file_search",
			"Search files attached to the current conversation. Use this tool when the user asks about a historical or uploaded file. Prefer file_key: pass the exact URI/file_key from the attachment metadata. Use file_name only as a compatibility fallback; when several uploads have the same name, the newest matching attachment is selected. Leave both empty only when searching all conversation files. The backend reads from its own storage, uses the parse cache when available, and parses on a cache miss. Do not invent file URLs.",
			func(ctx context.Context, input *fileSearchInput) (string, error) {
				if input == nil {
					return "query is required", nil
				}
				fileRef := strings.TrimSpace(input.FileKey)
				if fileRef == "" {
					fileRef = strings.TrimSpace(input.FileName)
				}
				return conf.Search(ctx, conf.ConversationID, conf.AgentID, conf.UserID, fileRef, input.Query)
			},
		)
		if err != nil {
			return nil, err
		}
		searchTool = withRecoverableFileTool(searchTool, toolActivityMeta{
			ToolName:     "file_search",
			RunningText:  "正在检索历史附件",
			FinishedText: "历史附件检索已结束",
			FailedText:   "历史附件检索失败",
		})
		tools = append(tools, searchTool)
	}

	if conf.Download != nil {
		downloadTool, err := utils.InferTool(
			"file_download",
			"Download an uploaded file from the current conversation into the current Skill workspace. Use this when a Skill script needs the original binary file, such as an Excel or Word document. Prefer file_key: pass the exact URI/file_key from the attachment metadata, because filenames are not unique. Use file_name only as a compatibility fallback when it identifies exactly one file. If no exact file_key or filename is visible in the current context, do not invent one; explain that the historical attachment cannot be identified and ask the user to upload it again or provide its exact filename. If multiple files have the same name, do not guess. The returned path is relative to the script workspace and can be passed directly to script_run. Do not invent file paths or URLs.",
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
		downloadTool = withRecoverableFileTool(downloadTool, toolActivityMeta{
			ToolName:     "file_download",
			RunningText:  "正在准备附件",
			FinishedText: "附件已准备",
			FailedText:   "附件准备失败",
		})
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
					return "", err
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
		uploadTool = withRecoverableFileTool(uploadTool, toolActivityMeta{
			ToolName:     "file_upload",
			RunningText:  "正在准备下载文件",
			FinishedText: "下载文件已准备",
			FailedText:   "下载文件准备失败",
		})
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

func withRecoverableFileTool(base tool.InvokableTool, activity toolActivityMeta) tool.InvokableTool {
	tracked := withToolActivity(base, activity)
	return utils.WrapInvokableToolWithErrorHandler(tracked, func(ctx context.Context, err error) string {
		logs.CtxWarnf(ctx, "recoverable file tool failure, tool=%s, err=%v", activity.ToolName, err)
		return modelFacingFileToolFailure(activity.ToolName, err)
	})
}

func modelFacingFileToolFailure(toolName string, err error) string {
	reason := strings.ToLower(err.Error())
	switch {
	case strings.Contains(reason, "no matching files found"):
		return "未能在当前会话中定位到所需附件。不要向用户展示或编造 file_key 等内部标识，也不要使用虚假标识重试。如果已知准确文件名，可以改用 file_name 重试；否则请自然说明暂时找不到之前上传的附件，并请用户重新上传或提供准确文件名后再继续。"
	case strings.Contains(reason, "matches") && strings.Contains(reason, "files"):
		return "存在多个同名附件，当前无法可靠判断用户指的是哪一份。不要猜测或向用户展示内部标识；请自然地请用户确认目标文件，或重新上传需要处理的版本。"
	case strings.Contains(reason, "file_key or file_name is required"):
		return "缺少可用的附件标识。不要编造文件标识；请根据上下文使用准确文件名，无法确认时请用户重新上传或说明目标文件。"
	case strings.Contains(reason, "too large"):
		return "附件超过当前支持的大小限制。请用自然语言说明暂时无法处理，并请用户提供体积更小的文件。"
	case strings.Contains(reason, "not supported") || strings.Contains(reason, "cannot determine uploaded file extension"):
		return "附件格式暂不支持或无法识别。请用自然语言说明限制，并请用户提供受支持格式的文件。"
	case toolName == "file_upload":
		return "生成文件暂时无法上传交付。请自然地说明文件准备失败并建议稍后重试；不要编造本地路径、对象存储地址或 Markdown 下载链接。"
	default:
		return "附件操作暂时无法完成。请用自然语言向用户说明，并建议重试或重新上传文件；不要反复使用编造的标识重试，也不要暴露内部错误信息。"
	}
}
