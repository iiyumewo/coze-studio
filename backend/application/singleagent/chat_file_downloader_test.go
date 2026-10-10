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
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	crossmessage "github.com/coze-dev/coze-studio/backend/crossdomain/message"
	"github.com/coze-dev/coze-studio/backend/crossdomain/message/messagemock"
	messagemodel "github.com/coze-dev/coze-studio/backend/crossdomain/message/model"
)

func TestFindConversationFilePrefersExactFileKey(t *testing.T) {
	response := &crossmessage.MessageListResponse{Messages: []*crossmessage.WfMessage{{
		MultiContent: []*crossmessage.Content{
			{Type: messagemodel.InputTypeFile, Uri: stringPtr("uploads/first/report.xlsx"), Name: stringPtr("report.xlsx")},
			{Type: messagemodel.InputTypeFile, Uri: stringPtr("uploads/second/report.xlsx"), Name: stringPtr("report.xlsx")},
		},
	}}}

	file, err := findConversationFile(response, "uploads/second/report.xlsx")
	require.NoError(t, err)
	require.Equal(t, "uploads/second/report.xlsx", file.URI)
}

func TestFindConversationFileRejectsAmbiguousFileNameWithCandidates(t *testing.T) {
	response := &crossmessage.MessageListResponse{Messages: []*crossmessage.WfMessage{{
		MultiContent: []*crossmessage.Content{
			{Type: messagemodel.InputTypeFile, Uri: stringPtr("uploads/first/report.xlsx"), Name: stringPtr("report.xlsx")},
			{Type: messagemodel.InputTypeFile, Uri: stringPtr("uploads/second/report.xlsx"), Name: stringPtr("report.xlsx")},
		},
	}}}

	_, err := findConversationFile(response, "report.xlsx")
	require.Error(t, err)
	require.Contains(t, err.Error(), "provide an exact file_key")
	require.Contains(t, err.Error(), "uploads/first/report.xlsx")
	require.Contains(t, err.Error(), "uploads/second/report.xlsx")
}

func TestChatFileDownloaderDownloadsOriginalFileToConversationWorkspace(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv("COZE_SKILL_RUNNER_WORKSPACE_ROOT", workspaceRoot)

	ctrl := gomock.NewController(t)
	messageService := messagemock.NewMockMessage(ctrl)
	messageService.EXPECT().MessageList(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *crossmessage.MessageListRequest) (*crossmessage.MessageListResponse, error) {
		require.Equal(t, "173381235121909759:1", req.UserID)
		return &crossmessage.MessageListResponse{
			Messages: []*crossmessage.WfMessage{{
				MultiContent: []*crossmessage.Content{{
					Type: messagemodel.InputTypeFile,
					Uri:  stringPtr("uploads/report.xlsx"),
					Name: stringPtr("report.xlsx"),
				}},
			}},
		}, nil
	})
	crossmessage.SetDefaultSVC(messageService)
	t.Cleanup(func() { crossmessage.SetDefaultSVC(nil) })

	downloader := newChatFileDownloader(&fakeChatFileStorage{body: []byte("xlsx bytes")})
	path, err := downloader(context.Background(), 123, 456, "173381235121909759:1", "report.xlsx")
	require.NoError(t, err)
	require.Equal(t, "report.xlsx", path)
	body, err := os.ReadFile(filepath.Join(workspaceRoot, "123", "report.xlsx"))
	require.NoError(t, err)
	require.Equal(t, "xlsx bytes", string(body))
}

func TestChatFileDownloaderUsesExactFileKeyWhenNamesDuplicate(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv("COZE_SKILL_RUNNER_WORKSPACE_ROOT", workspaceRoot)

	ctrl := gomock.NewController(t)
	messageService := messagemock.NewMockMessage(ctrl)
	messageService.EXPECT().MessageList(gomock.Any(), gomock.Any()).Return(&crossmessage.MessageListResponse{
		Messages: []*crossmessage.WfMessage{{
			MultiContent: []*crossmessage.Content{
				{Type: messagemodel.InputTypeFile, Uri: stringPtr("uploads/first/report.xlsx"), Name: stringPtr("report.xlsx")},
				{Type: messagemodel.InputTypeFile, Uri: stringPtr("uploads/second/report.xlsx"), Name: stringPtr("report.xlsx")},
			},
		}},
	}, nil)
	crossmessage.SetDefaultSVC(messageService)
	t.Cleanup(func() { crossmessage.SetDefaultSVC(nil) })

	fileStorage := &fakeChatFileStorage{body: []byte("second xlsx bytes")}
	downloader := newChatFileDownloader(fileStorage)
	path, err := downloader(context.Background(), 123, 456, "789", "uploads/second/report.xlsx")
	require.NoError(t, err)
	require.Equal(t, "report.xlsx", path)
	require.Equal(t, "uploads/second/report.xlsx", fileStorage.key)
}

func TestSafeWorkspaceFilenameRejectsPathTraversal(t *testing.T) {
	_, err := safeWorkspaceFilename("../report.xlsx", "uploads/report.xlsx")
	require.NoError(t, err)
	_, err = safeWorkspaceFilename("..", "uploads/report.xlsx")
	require.Error(t, err)
	_, err = safeWorkspaceFilename("report\x00.xlsx", "uploads/report.xlsx")
	require.Error(t, err)
}
