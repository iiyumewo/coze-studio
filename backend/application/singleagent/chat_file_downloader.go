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
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	crossmessage "github.com/coze-dev/coze-studio/backend/crossdomain/message"
	messagemodel "github.com/coze-dev/coze-studio/backend/crossdomain/message/model"
	fileentity "github.com/coze-dev/coze-studio/backend/domain/agent/singleagent/entity"
	"github.com/coze-dev/coze-studio/backend/infra/skill"
	"github.com/coze-dev/coze-studio/backend/infra/storage"
	"github.com/coze-dev/coze-studio/backend/pkg/envkey"
	"github.com/coze-dev/coze-studio/backend/types/consts"
)

const defaultChatFileDownloadMaxSize = 50 * 1024 * 1024

type chatFileDownloader struct {
	storage       storage.Storage
	workspaceRoot string
	maxFileSize   int
}

func newChatFileDownloader(objectStorage storage.Storage) fileentity.FileDownloadFunc {
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
	return (&chatFileDownloader{
		storage:       objectStorage,
		workspaceRoot: workspaceRoot,
		maxFileSize:   defaultChatFileDownloadMaxSize,
	}).DownloadConversation
}

func (d *chatFileDownloader) DownloadConversation(
	ctx context.Context,
	conversationID int64,
	agentID int64,
	userID string,
	fileRef string,
) (string, error) {
	if conversationID <= 0 {
		return "", fmt.Errorf("conversation id must be positive")
	}
	if strings.TrimSpace(fileRef) == "" {
		return "", fmt.Errorf("file_key or file_name is required")
	}
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("file download user id is invalid")
	}

	messageList, err := crossmessage.DefaultSVC().MessageList(ctx, &crossmessage.MessageListRequest{
		ConversationID: conversationID,
		Limit:          100,
		UserID:         uid,
		BizID:          agentID,
		OrderBy:        stringPtr("ASC"),
	})
	if err != nil {
		return "", fmt.Errorf("list conversation files failed: %w", err)
	}

	file, err := findConversationFile(messageList, fileRef)
	if err != nil {
		return "", err
	}
	filename, err := safeWorkspaceFilename(file.Name, file.URI)
	if err != nil {
		return "", err
	}

	workspaceDir := filepath.Join(d.workspaceRoot, strconv.FormatInt(conversationID, 10))
	if err := ensureWithinRoot(d.workspaceRoot, workspaceDir); err != nil {
		return "", fmt.Errorf("workspace path is invalid: %w", err)
	}
	if err := os.MkdirAll(workspaceDir, 0o750); err != nil {
		return "", fmt.Errorf("create Skill workspace failed: %w", err)
	}

	body, err := d.storage.GetObject(ctx, file.URI)
	if err != nil {
		return "", fmt.Errorf("download uploaded file failed: %w", err)
	}
	if len(body) > d.maxFileSize {
		return "", fmt.Errorf("uploaded file is too large: %d bytes", len(body))
	}

	destination := filepath.Join(workspaceDir, filename)
	if err := ensureWithinRoot(workspaceDir, destination); err != nil {
		return "", fmt.Errorf("download filename is invalid: %w", err)
	}
	if err := writeWorkspaceFile(destination, body); err != nil {
		return "", err
	}
	return filename, nil
}

type conversationFile struct {
	URI  string
	Name string
}

func findConversationFile(response *crossmessage.MessageListResponse, fileRef string) (conversationFile, error) {
	needle := strings.ToLower(strings.TrimSpace(fileRef))
	if needle == "" {
		return conversationFile{}, fmt.Errorf("file_key or file_name is required")
	}
	var files []conversationFile
	seen := make(map[string]struct{})
	if response != nil {
		for _, message := range response.Messages {
			for _, content := range message.MultiContent {
				if content == nil || content.Type != messagemodel.InputTypeFile || content.Uri == nil || *content.Uri == "" {
					continue
				}
				uri := *content.Uri
				if _, ok := seen[uri]; ok {
					continue
				}
				seen[uri] = struct{}{}
				name := ""
				if content.Name != nil {
					name = strings.TrimSpace(*content.Name)
				}
				if name == "" {
					name = path.Base(strings.Split(uri, "?")[0])
				}
				files = append(files, conversationFile{URI: uri, Name: name})
			}
		}
	}

	// Object storage keys are the authoritative identity. Resolve them before
	// considering filename compatibility, so repeated uploads are deterministic.
	for _, file := range files {
		if file.URI == strings.TrimSpace(fileRef) {
			return file, nil
		}
	}

	var exactNameMatches []conversationFile
	for _, file := range files {
		if strings.ToLower(file.Name) == needle {
			exactNameMatches = append(exactNameMatches, file)
		}
	}
	if len(exactNameMatches) == 1 {
		return exactNameMatches[0], nil
	}
	if len(exactNameMatches) > 1 {
		return conversationFile{}, ambiguousFileRefError(fileRef, exactNameMatches)
	}

	var matches []conversationFile
	for _, file := range files {
		if strings.Contains(strings.ToLower(file.Name), needle) || strings.Contains(strings.ToLower(file.URI), needle) {
			matches = append(matches, file)
		}
	}
	if len(matches) == 0 {
		return conversationFile{}, fmt.Errorf("no matching files found in current conversation")
	}
	if len(matches) > 1 {
		return conversationFile{}, ambiguousFileRefError(fileRef, matches)
	}
	return matches[0], nil
}

func ambiguousFileRefError(fileRef string, matches []conversationFile) error {
	keys := make([]string, 0, len(matches))
	for _, match := range matches {
		keys = append(keys, match.URI)
	}
	return fmt.Errorf("file reference %q matches %d files; provide an exact file_key, candidates: %s", fileRef, len(matches), strings.Join(keys, ", "))
}

func safeWorkspaceFilename(name, uri string) (string, error) {
	filename := strings.TrimSpace(name)
	if filename == "" {
		filename = path.Base(strings.Split(uri, "?")[0])
	}
	filename = path.Base(filename)
	if filename == "." || filename == ".." || filename == "" || strings.ContainsAny(filename, `/\\`) {
		return "", fmt.Errorf("uploaded filename is invalid")
	}
	for _, r := range filename {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("uploaded filename contains control characters")
		}
	}
	return filename, nil
}

func ensureWithinRoot(root, target string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path escapes workspace root")
	}
	return nil
}

func writeWorkspaceFile(destination string, body []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".coze-download-*")
	if err != nil {
		return fmt.Errorf("create download temp file failed: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	// Docker Runner mounts this host directory into a non-root container user;
	// keep the file readable after the runner changes the workspace directory
	// ownership to its sandbox UID.
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set download file permissions failed: %w", err)
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write downloaded file failed: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close downloaded file failed: %w", err)
	}
	if err := os.Rename(tmpName, destination); err != nil {
		return fmt.Errorf("publish downloaded file failed: %w", err)
	}
	return nil
}

func stringPtr(value string) *string { return &value }

var _ fileentity.FileDownloadFunc = (*chatFileDownloader)(nil).DownloadConversation
