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

package agentflow

import (
	"context"
	"errors"
	"testing"

	singleagent "github.com/coze-dev/coze-studio/backend/crossdomain/agent/model"
	"github.com/coze-dev/coze-studio/backend/domain/agent/singleagent/entity"
	"github.com/stretchr/testify/require"
)

func TestNewFileSearchToolsIncludesDownloadTool(t *testing.T) {
	var refs []string
	tools, err := newFileSearchTools(context.Background(), &fileSearchToolConfig{
		ConversationID: 42,
		AgentID:        7,
		UserID:         "9",
		Download: func(_ context.Context, conversationID, agentID int64, userID, fileRef string) (string, error) {
			require.Equal(t, int64(42), conversationID)
			require.Equal(t, int64(7), agentID)
			require.Equal(t, "9", userID)
			refs = append(refs, fileRef)
			return "report.xlsx", nil
		},
	})
	require.NoError(t, err)
	require.Len(t, tools, 1)

	info, err := tools[0].Info(context.Background())
	require.NoError(t, err)
	require.Equal(t, "file_download", info.Name)
	result, err := tools[0].InvokableRun(context.Background(), `{"file_key":"uploads/report.xlsx"}`)
	require.NoError(t, err)
	require.Equal(t, "report.xlsx", result)
	result, err = tools[0].InvokableRun(context.Background(), `{"file_name":"report.xlsx"}`)
	require.NoError(t, err)
	require.Equal(t, "report.xlsx", result)
	require.Equal(t, []string{"uploads/report.xlsx", "report.xlsx"}, refs)
}

func TestFileDownloadFailureIsReturnedToModelWithoutStoppingRun(t *testing.T) {
	tools, err := newFileSearchTools(context.Background(), &fileSearchToolConfig{
		Download: func(_ context.Context, _ int64, _ int64, _ string, _ string) (string, error) {
			return "", errors.New("no matching files found in current conversation")
		},
	})
	require.NoError(t, err)
	require.Len(t, tools, 1)

	events := make([]*singleagent.ToolActivityEvent, 0, 2)
	ctx := withToolActivityEmitter(context.Background(), func(event *singleagent.ToolActivityEvent) {
		events = append(events, event)
	})
	result, err := tools[0].InvokableRun(ctx, `{"file_key":"urn:file-key:invented"}`)

	require.NoError(t, err)
	require.Contains(t, result, "暂时找不到之前上传的附件")
	require.Contains(t, result, "不要向用户展示或编造 file_key")
	require.Len(t, events, 2)
	require.Equal(t, toolActivityPhaseStarted, events[0].Phase)
	require.Equal(t, toolActivityPhaseFailed, events[1].Phase)
}

func TestNewFileSearchToolsIncludesUploadTool(t *testing.T) {
	collector := entity.NewUploadedFileCollector()
	tools, err := newFileSearchTools(entity.WithUploadedFileCollector(context.Background(), collector), &fileSearchToolConfig{
		ConversationID: 42,
		AgentID:        7,
		UserID:         "9",
		Upload: func(_ context.Context, conversationID, agentID int64, userID, path string) (*entity.UploadedFile, error) {
			require.Equal(t, int64(42), conversationID)
			require.Equal(t, int64(7), agentID)
			require.Equal(t, "9", userID)
			require.Equal(t, "report.docx", path)
			return &entity.UploadedFile{FileKey: "artifacts/report.docx", Name: "report.docx", Type: "docx", Size: 4}, nil
		},
	})
	require.NoError(t, err)
	require.Len(t, tools, 1)
	info, err := tools[0].Info(context.Background())
	require.NoError(t, err)
	require.Equal(t, "file_upload", info.Name)
	result, err := tools[0].InvokableRun(entity.WithUploadedFileCollector(context.Background(), collector), `{"path":"report.docx"}`)
	require.NoError(t, err)
	require.Contains(t, result, "report.docx")
	require.Len(t, collector.Files(), 1)
}

func TestNewFileSearchToolsKeepsSearchToolOrder(t *testing.T) {
	tools, err := newFileSearchTools(context.Background(), &fileSearchToolConfig{
		Search: func(_ context.Context, _ int64, _ int64, _, _, _ string) (string, error) {
			return "content", nil
		},
		Download: func(_ context.Context, _ int64, _ int64, _ string, _ string) (string, error) {
			return "report.xlsx", nil
		},
	})
	require.NoError(t, err)
	require.Len(t, tools, 2)
	searchInfo, err := tools[0].Info(context.Background())
	require.NoError(t, err)
	downloadInfo, err := tools[1].Info(context.Background())
	require.NoError(t, err)
	require.Equal(t, "file_search", searchInfo.Name)
	require.Equal(t, "file_download", downloadInfo.Name)
}

func TestNewFileSearchToolPrefersFileKey(t *testing.T) {
	var fileRef string
	tools, err := newFileSearchTools(context.Background(), &fileSearchToolConfig{
		Search: func(_ context.Context, _ int64, _ int64, _, ref, query string) (string, error) {
			fileRef = ref
			require.Equal(t, "find rows", query)
			return "content", nil
		},
	})
	require.NoError(t, err)
	result, err := tools[0].InvokableRun(context.Background(), `{"file_key":"uploads/new/report.xlsx","file_name":"report.xlsx","query":"find rows"}`)
	require.NoError(t, err)
	require.Equal(t, "content", result)
	require.Equal(t, "uploads/new/report.xlsx", fileRef)
}
