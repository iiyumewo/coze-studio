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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatFileUploaderUploadsWorkspaceFile(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv("COZE_SKILL_RUNNER_WORKSPACE_ROOT", workspaceRoot)
	conversationDir := filepath.Join(workspaceRoot, "123")
	require.NoError(t, os.MkdirAll(conversationDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(conversationDir, "report.docx"), []byte("docx"), 0o640))

	storageImpl := &fakeChatFileStorage{}
	uploader := newChatFileUploader(storageImpl)
	file, err := uploader(context.Background(), 123, 456, "789", "report.docx")
	require.NoError(t, err)
	require.Equal(t, "report.docx", file.Name)
	require.Equal(t, "docx", file.Type)
	require.Equal(t, int64(4), file.Size)
	require.True(t, strings.HasPrefix(file.FileKey, "skill-artifacts/123/"))
	require.Equal(t, []byte("docx"), storageImpl.body)
}

func TestChatFileUploaderRejectsWorkspaceEscape(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv("COZE_SKILL_RUNNER_WORKSPACE_ROOT", workspaceRoot)
	uploader := newChatFileUploader(&fakeChatFileStorage{})
	_, err := uploader(context.Background(), 123, 456, "789", "../secret.txt")
	require.ErrorContains(t, err, "relative to the Skill workspace")
}
