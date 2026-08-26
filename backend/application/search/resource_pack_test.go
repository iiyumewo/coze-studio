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

package search

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-studio/backend/api/model/resource/common"
	mcpentity "github.com/coze-dev/coze-studio/backend/domain/mcp/entity"
)

type fakeMCPServerReader struct {
	server *mcpentity.MCPServer
	err    error
	id     int64
	calls  int
}

func (f *fakeMCPServerReader) GetByID(_ context.Context, id int64) (*mcpentity.MCPServer, error) {
	f.calls++
	f.id = id
	return f.server, f.err
}

func TestMCPPacker(t *testing.T) {
	const serverID int64 = 12345

	t.Run("pack MCP server data", func(t *testing.T) {
		reader := &fakeMCPServerReader{server: &mcpentity.MCPServer{
			ID:              serverID,
			Description:     "Browser automation tools",
			ServerURL:       "http://127.0.0.1:8931/mcp",
			Transport:       mcpentity.TransportTypeStreamableHTTP,
			Tools:           []*mcpentity.Tool{{Name: "browser_navigate"}, {Name: "browser_click"}},
			ProtocolVersion: "2025-06-18",
			ServerVersion:   "0.0.79",
			Status:          mcpentity.ServerStatusEnabled,
		}}
		packer, err := NewResourcePacker(serverID, common.ResType_MCP, &ServiceComponents{
			MCPServerReader: reader,
		})
		require.NoError(t, err)

		data, err := packer.GetDataInfo(context.Background())
		require.NoError(t, err)
		require.Equal(t, 1, reader.calls)
		require.Equal(t, serverID, reader.id)
		require.Equal(t, "Browser automation tools", *data.desc)
		require.Equal(t, int32(mcpentity.ServerStatusEnabled), *data.status)
		require.False(t, *data.detailDisable)
		require.Equal(t, "2", data.bizExtend["tool_count"])
		require.Equal(t, "streamable_http", data.bizExtend["transport"])
		require.Equal(t, "http://127.0.0.1:8931/mcp", data.bizExtend["server_url"])
		actions := packer.GetActions(context.Background())
		require.Len(t, actions, 1)
		require.Equal(t, common.ActionKey_Delete, actions[0].Key)
		require.True(t, actions[0].Enable)
	})

	t.Run("return repository error", func(t *testing.T) {
		repositoryErr := errors.New("database unavailable")
		packer, err := NewResourcePacker(serverID, common.ResType_MCP, &ServiceComponents{
			MCPServerReader: &fakeMCPServerReader{err: repositoryErr},
		})
		require.NoError(t, err)

		data, err := packer.GetDataInfo(context.Background())
		require.ErrorIs(t, err, repositoryErr)
		require.Nil(t, data)
	})
}
