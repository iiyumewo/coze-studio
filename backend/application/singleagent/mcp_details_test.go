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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-studio/backend/api/model/app/bot_common"
	agentmodel "github.com/coze-dev/coze-studio/backend/crossdomain/agent/model"
	"github.com/coze-dev/coze-studio/backend/domain/agent/singleagent/entity"
	mcpentity "github.com/coze-dev/coze-studio/backend/domain/mcp/entity"
)

func TestFetchMCPServerDetails(t *testing.T) {
	const (
		spaceID  = int64(100)
		serverID = int64(200)
	)

	reader := &fakeMCPServerReader{getByID: func(id int64) (*mcpentity.MCPServer, error) {
		require.Equal(t, serverID, id)
		return &mcpentity.MCPServer{
			ID:      id,
			SpaceID: spaceID,
			Name:    "filesystem",
			Status:  mcpentity.ServerStatusEnabled,
			Tools:   []*mcpentity.Tool{{Name: "read_file"}, {Name: "write_file"}},
		}, nil
	}}
	service := &SingleAgentApplicationService{appContext: &ServiceComponents{MCPServerReader: reader}}

	servers, err := service.fetchMCPServerDetails(context.Background(), &entity.SingleAgent{
		SingleAgent: &agentmodel.SingleAgent{
			SpaceID: spaceID,
			MCP: []*bot_common.MCPInfo{
				nil,
				{MCPServerID: 0},
				{MCPServerID: serverID},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, servers, 1)

	detail := mcpServerInfoDo2Vo(servers)[serverID]
	require.Equal(t, serverID, detail.GetID())
	require.Equal(t, "filesystem", detail.GetName())
	require.Equal(t, int32(2), detail.GetToolCount())
	require.Equal(t, int32(mcpentity.ServerStatusEnabled), detail.GetStatus())
}
