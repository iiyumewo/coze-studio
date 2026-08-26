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

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	mcpmodel "github.com/coze-dev/coze-studio/backend/api/model/mcp"
	mcpentity "github.com/coze-dev/coze-studio/backend/domain/mcp/entity"
	pluginencrypt "github.com/coze-dev/coze-studio/backend/domain/plugin/encrypt"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

type fakeGetMCPServerRepository struct {
	getByIDFn func(ctx context.Context, id int64) (*mcpentity.MCPServer, error)
	calls     int
	id        int64
}

func (f *fakeGetMCPServerRepository) Create(
	context.Context,
	*mcpentity.MCPServer,
) (*mcpentity.MCPServer, error) {
	return nil, errors.New("unexpected Create call")
}

func (f *fakeGetMCPServerRepository) GetByID(
	ctx context.Context,
	id int64,
) (*mcpentity.MCPServer, error) {
	f.calls++
	f.id = id
	return f.getByIDFn(ctx, id)
}

func (f *fakeGetMCPServerRepository) Delete(context.Context, int64, int64) error {
	return errors.New("unexpected Delete call")
}

func TestMCPApplicationServiceGetMCPServer(t *testing.T) {
	t.Run("return workspace MCP detail without authentication secret", func(t *testing.T) {
		t.Setenv(pluginencrypt.AuthSecretEnv, testAESSecret)
		encryptedAuth := encryptedHeaderAuthentication(t, "Authorization", testAuthValue)
		repository := &fakeGetMCPServerRepository{
			getByIDFn: func(context.Context, int64) (*mcpentity.MCPServer, error) {
				return &mcpentity.MCPServer{
					ID:               testServerID,
					SpaceID:          testSpaceID,
					Name:             "Playwright",
					Description:      "Browser automation",
					ServerURL:        "http://127.0.0.1:8931/mcp",
					Transport:        mcpentity.TransportTypeStreamableHTTP,
					AuthType:         mcpentity.AuthenticationTypeHeader,
					EncryptedAuth:    &encryptedAuth,
					ProtocolVersion:  "2025-06-18",
					ServerVersion:    "0.0.79",
					Status:           mcpentity.ServerStatusEnabled,
					ValidationStatus: mcpentity.ValidationStatusSuccess,
					LastCheckedAt:    100,
					CreatedAt:        200,
					UpdatedAt:        300,
					Tools: []*mcpentity.Tool{
						{Name: "browser_navigate", Title: "Navigate", Description: "Open a URL"},
					},
				}, nil
			},
		}
		service := newApplicationService(nil, repository, nil)

		response, err := service.GetMCPServer(authorizedContext(t, testSpaceID), &mcpmodel.GetMCPServerRequest{
			MCPServerID: testServerID,
			SpaceID:     testSpaceID,
		})

		require.NoError(t, err)
		require.Equal(t, 1, repository.calls)
		require.Equal(t, testServerID, repository.id)
		require.NotNil(t, response)
		require.Equal(t, int64(0), response.Code)
		require.Equal(t, "success", response.Msg)
		require.NotNil(t, response.BaseResp)
		require.Equal(t, "Playwright", response.Data.Name)
		require.Equal(t, "http://127.0.0.1:8931/mcp", response.Data.ServerURL)
		require.Equal(t, "header", response.Data.Authentication.Type)
		require.Equal(t, "Authorization", response.Data.Authentication.GetHeaderName())
		require.Len(t, response.Data.Tools, 1)
		require.Equal(t, "browser_navigate", response.Data.Tools[0].Name)

		responseJSON, err := json.Marshal(response)
		require.NoError(t, err)
		require.NotContains(t, string(responseJSON), testAuthValue)
	})

	t.Run("reject MCP server from another workspace", func(t *testing.T) {
		repository := &fakeGetMCPServerRepository{
			getByIDFn: func(context.Context, int64) (*mcpentity.MCPServer, error) {
				return &mcpentity.MCPServer{
					ID:      testServerID,
					SpaceID: testSpaceID + 1,
				}, nil
			},
		}
		service := newApplicationService(nil, repository, nil)

		response, err := service.GetMCPServer(authorizedContext(t, testSpaceID), &mcpmodel.GetMCPServerRequest{
			MCPServerID: testServerID,
			SpaceID:     testSpaceID,
		})

		require.Nil(t, response)
		requireStatusErrorCode(t, err, errno.ErrMCPPermissionCode)
		require.Equal(t, 1, repository.calls)
	})

	t.Run("reject inaccessible workspace before querying repository", func(t *testing.T) {
		repository := &fakeGetMCPServerRepository{
			getByIDFn: func(context.Context, int64) (*mcpentity.MCPServer, error) {
				t.Fatal("repository must not be called when workspace access is denied")
				return nil, nil
			},
		}
		service := newApplicationService(nil, repository, nil)

		response, err := service.GetMCPServer(authorizedContext(t, testSpaceID+1), &mcpmodel.GetMCPServerRequest{
			MCPServerID: testServerID,
			SpaceID:     testSpaceID,
		})

		require.Nil(t, response)
		requireStatusErrorCode(t, err, errno.ErrMCPPermissionCode)
		require.Equal(t, 0, repository.calls)
	})

	t.Run("map repository failure to MCP get error", func(t *testing.T) {
		repository := &fakeGetMCPServerRepository{
			getByIDFn: func(context.Context, int64) (*mcpentity.MCPServer, error) {
				return nil, errors.New("database unavailable")
			},
		}
		service := newApplicationService(nil, repository, nil)

		response, err := service.GetMCPServer(authorizedContext(t, testSpaceID), &mcpmodel.GetMCPServerRequest{
			MCPServerID: testServerID,
			SpaceID:     testSpaceID,
		})

		require.Nil(t, response)
		requireStatusErrorCode(t, err, errno.ErrMCPGetCode)
	})

	t.Run("reject unreadable encrypted authentication", func(t *testing.T) {
		invalidEncryptedAuth := "invalid"
		repository := &fakeGetMCPServerRepository{
			getByIDFn: func(context.Context, int64) (*mcpentity.MCPServer, error) {
				return &mcpentity.MCPServer{
					ID:            testServerID,
					SpaceID:       testSpaceID,
					AuthType:      mcpentity.AuthenticationTypeHeader,
					EncryptedAuth: &invalidEncryptedAuth,
				}, nil
			},
		}
		service := newApplicationService(nil, repository, nil)

		response, err := service.GetMCPServer(authorizedContext(t, testSpaceID), &mcpmodel.GetMCPServerRequest{
			MCPServerID: testServerID,
			SpaceID:     testSpaceID,
		})

		require.Nil(t, response)
		requireStatusErrorCode(t, err, errno.ErrMCPGetCode)
	})
}

func encryptedHeaderAuthentication(t *testing.T, headerName, headerValue string) string {
	t.Helper()
	payload, err := json.Marshal(&headerAuthenticationPayload{
		HeaderName:  headerName,
		HeaderValue: headerValue,
	})
	require.NoError(t, err)

	encryptedAuth, err := pluginencrypt.EncryptByAES(payload, testAESSecret)
	require.NoError(t, err)
	return encryptedAuth
}
