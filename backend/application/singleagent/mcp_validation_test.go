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

package singleagent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-studio/backend/api/model/app/bot_common"
	mcpentity "github.com/coze-dev/coze-studio/backend/domain/mcp/entity"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

type fakeMCPServerReader struct {
	getByID func(id int64) (*mcpentity.MCPServer, error)
	calls   int
}

func (f *fakeMCPServerReader) GetByID(_ context.Context, id int64) (*mcpentity.MCPServer, error) {
	f.calls++
	return f.getByID(id)
}

func (f *fakeMCPServerReader) MGetByIDs(_ context.Context, spaceID int64, ids []int64) ([]*mcpentity.MCPServer, error) {
	servers := make([]*mcpentity.MCPServer, 0, len(ids))
	for _, id := range ids {
		server, err := f.getByID(id)
		if err != nil {
			return nil, err
		}
		if server != nil && server.SpaceID == spaceID {
			servers = append(servers, server)
		}
	}
	return servers, nil
}

func TestValidateMCPServers(t *testing.T) {
	const (
		spaceID  int64 = 100
		serverID int64 = 200
	)

	t.Run("accept enabled servers from the agent space", func(t *testing.T) {
		reader := &fakeMCPServerReader{getByID: func(id int64) (*mcpentity.MCPServer, error) {
			return &mcpentity.MCPServer{ID: id, SpaceID: spaceID, Status: mcpentity.ServerStatusEnabled}, nil
		}}
		service := &SingleAgentApplicationService{appContext: &ServiceComponents{MCPServerReader: reader}}

		err := service.validateMCPServers(context.Background(), spaceID, []*bot_common.MCPInfo{{MCPServerID: serverID}})

		require.NoError(t, err)
		require.Equal(t, 1, reader.calls)
	})

	t.Run("allow an empty list to clear the configuration", func(t *testing.T) {
		reader := &fakeMCPServerReader{getByID: func(int64) (*mcpentity.MCPServer, error) {
			return nil, errors.New("unexpected query")
		}}
		service := &SingleAgentApplicationService{appContext: &ServiceComponents{MCPServerReader: reader}}

		err := service.validateMCPServers(context.Background(), spaceID, []*bot_common.MCPInfo{})

		require.NoError(t, err)
		require.Zero(t, reader.calls)
	})

	t.Run("reject invalid and duplicate IDs before querying", func(t *testing.T) {
		testCases := []struct {
			name string
			refs []*bot_common.MCPInfo
		}{
			{name: "missing reference", refs: []*bot_common.MCPInfo{nil}},
			{name: "zero ID", refs: []*bot_common.MCPInfo{{MCPServerID: 0}}},
			{name: "duplicate ID", refs: []*bot_common.MCPInfo{{MCPServerID: serverID}, {MCPServerID: serverID}}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				reader := &fakeMCPServerReader{getByID: func(int64) (*mcpentity.MCPServer, error) {
					return nil, errors.New("unexpected query")
				}}
				service := &SingleAgentApplicationService{appContext: &ServiceComponents{MCPServerReader: reader}}

				err := service.validateMCPServers(context.Background(), spaceID, tc.refs)

				requireStatusErrorCode(t, err, errno.ErrMCPInvalidParamCode)
				require.Zero(t, reader.calls)
			})
		}
	})

	t.Run("reject a server from another space", func(t *testing.T) {
		reader := &fakeMCPServerReader{getByID: func(id int64) (*mcpentity.MCPServer, error) {
			return &mcpentity.MCPServer{ID: id, SpaceID: spaceID + 1, Status: mcpentity.ServerStatusEnabled}, nil
		}}
		service := &SingleAgentApplicationService{appContext: &ServiceComponents{MCPServerReader: reader}}

		err := service.validateMCPServers(context.Background(), spaceID, []*bot_common.MCPInfo{{MCPServerID: serverID}})

		requireStatusErrorCode(t, err, errno.ErrMCPPermissionCode)
	})

	t.Run("reject a disabled server", func(t *testing.T) {
		reader := &fakeMCPServerReader{getByID: func(id int64) (*mcpentity.MCPServer, error) {
			return &mcpentity.MCPServer{ID: id, SpaceID: spaceID, Status: mcpentity.ServerStatusDisabled}, nil
		}}
		service := &SingleAgentApplicationService{appContext: &ServiceComponents{MCPServerReader: reader}}

		err := service.validateMCPServers(context.Background(), spaceID, []*bot_common.MCPInfo{{MCPServerID: serverID}})

		requireStatusErrorCode(t, err, errno.ErrMCPValidateCode)
	})

	t.Run("return an MCP get error when the repository fails", func(t *testing.T) {
		reader := &fakeMCPServerReader{getByID: func(int64) (*mcpentity.MCPServer, error) {
			return nil, errors.New("database unavailable")
		}}
		service := &SingleAgentApplicationService{appContext: &ServiceComponents{MCPServerReader: reader}}

		err := service.validateMCPServers(context.Background(), spaceID, []*bot_common.MCPInfo{{MCPServerID: serverID}})

		requireStatusErrorCode(t, err, errno.ErrMCPGetCode)
	})
}

func requireStatusErrorCode(t *testing.T, err error, expectedCode int32) {
	t.Helper()
	require.Error(t, err)
	var statusErr errorx.StatusError
	require.ErrorAs(t, err, &statusErr)
	require.Equal(t, expectedCode, statusErr.Code())
}
