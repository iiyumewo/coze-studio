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
	crossuser "github.com/coze-dev/coze-studio/backend/crossdomain/user"
	mcpentity "github.com/coze-dev/coze-studio/backend/domain/mcp/entity"
	pluginencrypt "github.com/coze-dev/coze-studio/backend/domain/plugin/encrypt"
	userentity "github.com/coze-dev/coze-studio/backend/domain/user/entity"
	mcpinfra "github.com/coze-dev/coze-studio/backend/infra/mcp"
	"github.com/coze-dev/coze-studio/backend/pkg/ctxcache"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/types/consts"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

const (
	testUserID    = int64(1001)
	testSpaceID   = int64(2001)
	testServerID  = int64(3001)
	testAuthValue = "Bearer test-secret"
	testAESSecret = "0123456789abcdef"
)

type fakeMCPClient struct {
	testFn  func(ctx context.Context, request *mcpinfra.TestRequest) (*mcpinfra.TestResult, error)
	request *mcpinfra.TestRequest
	calls   int
}

func (f *fakeMCPClient) Test(ctx context.Context, request *mcpinfra.TestRequest) (*mcpinfra.TestResult, error) {
	f.calls++
	f.request = request
	return f.testFn(ctx, request)
}

type fakeMCPServerRepository struct {
	createFn func(ctx context.Context, server *mcpentity.MCPServer) (*mcpentity.MCPServer, error)
	server   *mcpentity.MCPServer
	calls    int
}

func (f *fakeMCPServerRepository) Create(
	ctx context.Context,
	server *mcpentity.MCPServer,
) (*mcpentity.MCPServer, error) {
	f.calls++
	f.server = server
	return f.createFn(ctx, server)
}

type fakeUserService struct {
	spaces []*userentity.Space
	err    error
}

func (f *fakeUserService) GetUserSpaceList(context.Context, int64) ([]*userentity.Space, error) {
	return f.spaces, f.err
}

func (f *fakeUserService) GetUserSpaceBySpaceID(context.Context, []int64) ([]*userentity.Space, error) {
	return nil, nil
}

func TestMCPApplicationServiceTestAndCreateMCPServer(t *testing.T) {
	t.Run("test encrypt save and respond successfully", func(t *testing.T) {
		t.Setenv(pluginencrypt.AuthSecretEnv, testAESSecret)
		ctx := authorizedContext(t, testSpaceID)
		client := &fakeMCPClient{
			testFn: func(context.Context, *mcpinfra.TestRequest) (*mcpinfra.TestResult, error) {
				return successfulTestResult(), nil
			},
		}
		repository := &fakeMCPServerRepository{
			createFn: func(_ context.Context, server *mcpentity.MCPServer) (*mcpentity.MCPServer, error) {
				createdServer := *server
				createdServer.ID = testServerID
				return &createdServer, nil
			},
		}
		service := newApplicationService(client, repository)

		response, err := service.TestAndCreateMCPServer(ctx, headerAuthenticationRequest())

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, int64(0), response.Code)
		require.Equal(t, "success", response.Msg)
		require.NotNil(t, response.BaseResp)
		require.Equal(t, testServerID, response.Data.ID)
		require.Equal(t, "Test MCP Server", response.Data.Name)
		require.Equal(t, int32(1), response.Data.ToolCount)

		require.Equal(t, 1, client.calls)
		require.Equal(t, "https://example.com/mcp", client.request.ServerURL)
		require.Equal(t, mcpinfra.AuthenticationTypeHeader, client.request.Authentication.Type)
		require.Equal(t, "Authorization", client.request.Authentication.HeaderName)
		require.Equal(t, testAuthValue, client.request.Authentication.HeaderValue)

		require.Equal(t, 1, repository.calls)
		require.Equal(t, testSpaceID, repository.server.SpaceID)
		require.Equal(t, testUserID, repository.server.CreatorID)
		require.Equal(t, mcpentity.AuthenticationTypeHeader, repository.server.AuthType)
		require.NotNil(t, repository.server.EncryptedAuth)
		require.NotContains(t, *repository.server.EncryptedAuth, testAuthValue)
		require.Len(t, repository.server.Tools, 1)
		require.Equal(t, "lookup", repository.server.Tools[0].Name)

		decryptedAuth, err := pluginencrypt.DecryptByAES(*repository.server.EncryptedAuth, testAESSecret)
		require.NoError(t, err)
		var authentication headerAuthenticationPayload
		require.NoError(t, json.Unmarshal(decryptedAuth, &authentication))
		require.Equal(t, "Authorization", authentication.HeaderName)
		require.Equal(t, testAuthValue, authentication.HeaderValue)
	})

	t.Run("reject workspace access before testing MCP", func(t *testing.T) {
		ctx := authorizedContext(t, testSpaceID+1)
		client := &fakeMCPClient{
			testFn: func(context.Context, *mcpinfra.TestRequest) (*mcpinfra.TestResult, error) {
				t.Fatal("MCP client must not be called when workspace access is denied")
				return nil, nil
			},
		}
		repository := &fakeMCPServerRepository{
			createFn: func(context.Context, *mcpentity.MCPServer) (*mcpentity.MCPServer, error) {
				t.Fatal("repository must not be called when workspace access is denied")
				return nil, nil
			},
		}
		service := newApplicationService(client, repository)

		response, err := service.TestAndCreateMCPServer(ctx, headerAuthenticationRequest())

		require.Nil(t, response)
		requireStatusErrorCode(t, err, errno.ErrMCPPermissionCode)
		require.Equal(t, 0, client.calls)
		require.Equal(t, 0, repository.calls)
	})

	t.Run("do not save when MCP validation fails", func(t *testing.T) {
		ctx := authorizedContext(t, testSpaceID)
		client := &fakeMCPClient{
			testFn: func(context.Context, *mcpinfra.TestRequest) (*mcpinfra.TestResult, error) {
				return nil, errors.New(mcpinfra.ErrConnectServer.Error())
			},
		}
		repository := &fakeMCPServerRepository{
			createFn: func(context.Context, *mcpentity.MCPServer) (*mcpentity.MCPServer, error) {
				t.Fatal("repository must not be called when MCP validation fails")
				return nil, nil
			},
		}
		service := newApplicationService(client, repository)

		response, err := service.TestAndCreateMCPServer(ctx, headerAuthenticationRequest())

		require.Nil(t, response)
		requireStatusErrorCode(t, err, errno.ErrMCPValidateCode)
		require.Equal(t, 1, client.calls)
		require.Equal(t, 0, repository.calls)
	})

	t.Run("map repository failure to MCP create error", func(t *testing.T) {
		ctx := authorizedContext(t, testSpaceID)
		client := &fakeMCPClient{
			testFn: func(context.Context, *mcpinfra.TestRequest) (*mcpinfra.TestResult, error) {
				return successfulTestResult(), nil
			},
		}
		repository := &fakeMCPServerRepository{
			createFn: func(context.Context, *mcpentity.MCPServer) (*mcpentity.MCPServer, error) {
				return nil, errors.New("database unavailable")
			},
		}
		service := newApplicationService(client, repository)

		response, err := service.TestAndCreateMCPServer(ctx, headerAuthenticationRequest())

		require.Nil(t, response)
		requireStatusErrorCode(t, err, errno.ErrMCPCreateCode)
		require.Equal(t, 1, client.calls)
		require.Equal(t, 1, repository.calls)
	})
}

func authorizedContext(t *testing.T, memberSpaceID int64) context.Context {
	t.Helper()
	previousUserService := crossuser.DefaultSVC()
	crossuser.SetDefaultSVC(&fakeUserService{
		spaces: []*userentity.Space{{ID: memberSpaceID}},
	})
	t.Cleanup(func() {
		crossuser.SetDefaultSVC(previousUserService)
	})

	ctx := ctxcache.Init(context.Background())
	ctxcache.Store(ctx, consts.SessionDataKeyInCtx, &userentity.Session{UserID: testUserID})
	return ctx
}

func headerAuthenticationRequest() *mcpmodel.TestAndCreateMCPServerRequest {
	headerName := " Authorization "
	headerValue := testAuthValue
	return &mcpmodel.TestAndCreateMCPServerRequest{
		SpaceID:   testSpaceID,
		ServerURL: " https://example.com/mcp ",
		Transport: " streamable_http ",
		Authentication: &mcpmodel.MCPServerAuthentication{
			Type:        "header",
			HeaderName:  &headerName,
			HeaderValue: &headerValue,
		},
	}
}

func successfulTestResult() *mcpinfra.TestResult {
	return &mcpinfra.TestResult{
		ServerName:      "test-mcp-server",
		ServerTitle:     "Test MCP Server",
		ServerVersion:   "1.0.0",
		ProtocolVersion: "2025-06-18",
		Instructions:    "Test server instructions",
		Tools: []*mcpinfra.Tool{
			{
				Name:        "lookup",
				Description: "Look up a record",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
		},
	}
}

func requireStatusErrorCode(t *testing.T, err error, expectedCode int32) {
	t.Helper()
	require.Error(t, err)
	var statusErr errorx.StatusError
	require.ErrorAs(t, err, &statusErr)
	require.Equal(t, expectedCode, statusErr.Code())
}
