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

/*
 * Copyright 2025 Coze Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 */

package singleagent

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	fileentity "github.com/coze-dev/coze-studio/backend/domain/agent/singleagent/entity"
	"github.com/coze-dev/coze-studio/backend/infra/skill"
	"github.com/coze-dev/coze-studio/backend/infra/storage"
	"github.com/coze-dev/coze-studio/backend/pkg/envkey"
	"github.com/coze-dev/coze-studio/backend/types/consts"
)

const defaultChatFileUploadMaxSize = 200 * 1024 * 1024

type chatFileUploader struct {
	storage       storage.Storage
	workspaceRoot string
	maxFileSize   int64
}

func newChatFileUploader(objectStorage storage.Storage) fileentity.FileUploadFunc {
	if objectStorage == nil {
		return nil
	}
	workspaceRoot := envkey.GetString(consts.SkillRunnerWorkspaceRoot)
	if strings.TrimSpace(workspaceRoot) == "" {
		workspaceRoot = skill.DefaultWorkspaceRoot()
	}
	workspaceRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil
	}
	return (&chatFileUploader{
		storage:       objectStorage,
		workspaceRoot: workspaceRoot,
		maxFileSize:   defaultChatFileUploadMaxSize,
	}).UploadWorkspaceFile
}

func (u *chatFileUploader) UploadWorkspaceFile(
	ctx context.Context,
	conversationID int64,
	agentID int64,
	userID string,
	relativePath string,
) (*fileentity.UploadedFile, error) {
	if conversationID <= 0 {
		return nil, fmt.Errorf("conversation id must be positive")
	}
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user id is required")
	}
	if strings.TrimSpace(relativePath) == "" {
		return nil, fmt.Errorf("path is required")
	}

	workspaceDir := filepath.Join(u.workspaceRoot, strconv.FormatInt(conversationID, 10))
	if err := ensureWithinRoot(u.workspaceRoot, workspaceDir); err != nil {
		return nil, fmt.Errorf("workspace path is invalid: %w", err)
	}
	cleanPath := filepath.Clean(relativePath)
	if filepath.IsAbs(relativePath) || cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("path must be relative to the Skill workspace")
	}
	sourcePath := filepath.Join(workspaceDir, cleanPath)
	if err := ensureWithinRoot(workspaceDir, sourcePath); err != nil {
		return nil, fmt.Errorf("path escapes Skill workspace")
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("workspace file not found: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("workspace path is a directory")
	}
	if info.Size() > u.maxFileSize {
		return nil, fmt.Errorf("workspace file is too large: %d bytes", info.Size())
	}
	body, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("read workspace file failed: %w", err)
	}

	name := filepath.Base(sourcePath)
	fileType := fileTypeForName(name)
	mimeType := mime.TypeByExtension(filepath.Ext(name))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	objectKey := fmt.Sprintf("skill-artifacts/%d/%s-%s", conversationID, uuid.NewString(), name)
	if err := u.storage.PutObject(ctx, objectKey, body,
		storage.WithContentType(mimeType),
		storage.WithContentDisposition(fmt.Sprintf("attachment; filename=%q", name)),
		storage.WithObjectSize(int64(len(body))),
	); err != nil {
		return nil, fmt.Errorf("store workspace file failed: %w", err)
	}
	url, err := u.storage.GetObjectUrl(ctx, objectKey, storage.WithExpire(24*60*60))
	if err != nil {
		return nil, fmt.Errorf("create file download URL failed: %w", err)
	}
	return &fileentity.UploadedFile{
		FileKey: objectKey,
		Name:    name,
		Type:    fileType,
		Size:    int64(len(body)),
		URL:     url,
	}, nil
}

func fileTypeForName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		return "pdf"
	case ".doc", ".docx":
		return "docx"
	case ".xls", ".xlsx":
		return "excel"
	case ".csv":
		return "csv"
	case ".txt", ".md":
		return "txt"
	case ".ppt", ".pptx":
		return "ppt"
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2":
		return "archive"
	case ".py", ".js", ".ts", ".java", ".c", ".cpp", ".html", ".css":
		return "code"
	case ".mp3", ".wav", ".aac", ".flac", ".ogg":
		return "audio"
	case ".mp4", ".avi", ".mov", ".wmv", ".flv", ".mkv":
		return "video"
	default:
		return "default_unknown"
	}
}
