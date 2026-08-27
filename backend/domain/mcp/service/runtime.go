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

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	crossmodel "github.com/coze-dev/coze-studio/backend/crossdomain/mcp/model"
	"github.com/coze-dev/coze-studio/backend/domain/mcp/entity"
	mcpinfra "github.com/coze-dev/coze-studio/backend/infra/mcp"
)

func (s *mcpServiceImpl) MGetAgentTools(
	ctx context.Context,
	req *crossmodel.MGetAgentToolsRequest,
) ([]*crossmodel.ToolInfo, error) {
	if req == nil || req.SpaceID <= 0 {
		return nil, errors.New("valid MCP agent tool request is required")
	}
	if len(req.MCPServerIDs) == 0 {
		return []*crossmodel.ToolInfo{}, nil
	}
	if s.components == nil || s.components.ServerRepo == nil {
		return nil, errors.New("MCP server repository is not initialized")
	}

	serverIDs, err := uniqueServerIDs(req.MCPServerIDs)
	if err != nil {
		return nil, err
	}
	servers, err := s.components.ServerRepo.MGetByIDs(ctx, req.SpaceID, serverIDs)
	if err != nil {
		return nil, fmt.Errorf("get MCP servers: %w", err)
	}

	serverMap := make(map[int64]*entity.MCPServer, len(servers))
	for _, server := range servers {
		if server != nil {
			serverMap[server.ID] = server
		}
	}

	tools := make([]*crossmodel.ToolInfo, 0)
	for _, serverID := range serverIDs {
		server := serverMap[serverID]
		if server == nil {
			return nil, fmt.Errorf("MCP server %d not found in workspace", serverID)
		}
		if err = validateRuntimeServer(server, req.SpaceID); err != nil {
			return nil, err
		}
		if len(server.Tools) == 0 {
			return nil, fmt.Errorf("MCP server %d declares no tools", serverID)
		}

		for _, serverTool := range server.Tools {
			if serverTool == nil || strings.TrimSpace(serverTool.Name) == "" {
				return nil, fmt.Errorf("MCP server %d contains an invalid tool", serverID)
			}
			tools = append(tools, &crossmodel.ToolInfo{
				MCPServerID:   server.ID,
				MCPServerName: server.Name,
				Name:          strings.TrimSpace(serverTool.Name),
				Description:   serverTool.Description,
				InputSchema:   append([]byte(nil), serverTool.InputSchema...),
			})
		}
	}

	return tools, nil
}

func (s *mcpServiceImpl) ExecuteTool(
	ctx context.Context,
	req *crossmodel.ExecuteToolRequest,
) (*crossmodel.ExecuteToolResponse, error) {
	if req == nil || req.SpaceID <= 0 || req.MCPServerID <= 0 || strings.TrimSpace(req.ToolName) == "" {
		return nil, errors.New("valid MCP tool execution request is required")
	}
	if s.components == nil || s.components.ServerRepo == nil || s.components.ToolClient == nil {
		return nil, errors.New("MCP runtime service is not initialized")
	}

	server, err := s.components.ServerRepo.GetByID(ctx, req.MCPServerID)
	if err != nil {
		return nil, fmt.Errorf("get MCP server %d: %w", req.MCPServerID, err)
	}
	if server == nil {
		return nil, fmt.Errorf("MCP server %d not found", req.MCPServerID)
	}
	if err = validateRuntimeServer(server, req.SpaceID); err != nil {
		return nil, err
	}

	toolName := strings.TrimSpace(req.ToolName)
	if !serverHasTool(server, toolName) {
		return nil, fmt.Errorf("tool %q is not declared by MCP server %d", toolName, server.ID)
	}

	authentication, err := mcpinfra.DecryptAuthentication(
		mcpinfra.AuthenticationType(server.AuthType),
		server.EncryptedAuth,
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt MCP server %d authentication: %w", server.ID, err)
	}

	result, err := s.components.ToolClient.CallTool(ctx, &mcpinfra.CallToolRequest{
		ServerURL:      server.ServerURL,
		Authentication: authentication,
		ToolName:       toolName,
		Arguments:      append([]byte(nil), req.Arguments...),
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("MCP tool client returned an empty result")
	}

	return &crossmodel.ExecuteToolResponse{
		Output:  result.Output,
		IsError: result.IsError,
	}, nil
}

func uniqueServerIDs(serverIDs []int64) ([]int64, error) {
	uniqueIDs := make([]int64, 0, len(serverIDs))
	seen := make(map[int64]struct{}, len(serverIDs))
	for _, serverID := range serverIDs {
		if serverID <= 0 {
			return nil, fmt.Errorf("MCP server ID must be greater than zero")
		}
		if _, exists := seen[serverID]; exists {
			return nil, fmt.Errorf("duplicate MCP server ID %d", serverID)
		}
		seen[serverID] = struct{}{}
		uniqueIDs = append(uniqueIDs, serverID)
	}

	return uniqueIDs, nil
}

func validateRuntimeServer(server *entity.MCPServer, spaceID int64) error {
	if server.SpaceID != spaceID {
		return fmt.Errorf("MCP server %d does not belong to workspace", server.ID)
	}
	if server.Status != entity.ServerStatusEnabled {
		return fmt.Errorf("MCP server %d is disabled", server.ID)
	}
	if server.ValidationStatus != entity.ValidationStatusSuccess {
		return fmt.Errorf("MCP server %d is not validated", server.ID)
	}
	if server.Transport != entity.TransportTypeStreamableHTTP {
		return fmt.Errorf("MCP server %d uses an unsupported transport", server.ID)
	}

	return nil
}

func serverHasTool(server *entity.MCPServer, toolName string) bool {
	for _, serverTool := range server.Tools {
		if serverTool != nil && strings.TrimSpace(serverTool.Name) == toolName {
			return true
		}
	}

	return false
}
