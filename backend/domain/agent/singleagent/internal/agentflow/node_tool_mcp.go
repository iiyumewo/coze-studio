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

package agentflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	einojsonschema "github.com/eino-contrib/jsonschema"

	"github.com/coze-dev/coze-studio/backend/api/model/app/bot_common"
	crossmcp "github.com/coze-dev/coze-studio/backend/crossdomain/mcp"
	"github.com/coze-dev/coze-studio/backend/crossdomain/mcp/model"
)

type mcpToolConfig struct {
	spaceID    int64
	serverRefs []*bot_common.MCPInfo
}

func newMCPTools(ctx context.Context, conf *mcpToolConfig) ([]tool.InvokableTool, error) {
	if conf == nil || len(conf.serverRefs) == 0 {
		return []tool.InvokableTool{}, nil
	}

	service := crossmcp.DefaultSVC()
	if service == nil {
		return nil, errors.New("MCP service is not initialized")
	}

	serverIDs := make([]int64, 0, len(conf.serverRefs))
	for _, serverRef := range conf.serverRefs {
		if serverRef == nil || serverRef.MCPServerID <= 0 {
			return nil, errors.New("agent contains an invalid MCP server reference")
		}
		serverIDs = append(serverIDs, serverRef.MCPServerID)
	}

	toolInfos, err := service.MGetAgentTools(ctx, &model.MGetAgentToolsRequest{
		SpaceID:      conf.spaceID,
		MCPServerIDs: serverIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("get MCP agent tools: %w", err)
	}

	tools := make([]tool.InvokableTool, 0, len(toolInfos))
	for _, toolInfo := range toolInfos {
		if toolInfo == nil {
			return nil, errors.New("MCP service returned an invalid tool")
		}
		tools = append(tools, &mcpInvokableTool{
			spaceID:  conf.spaceID,
			toolInfo: toolInfo,
		})
	}

	return tools, nil
}

type mcpInvokableTool struct {
	spaceID  int64
	toolInfo *model.ToolInfo
}

func (m *mcpInvokableTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	if m == nil || m.toolInfo == nil || strings.TrimSpace(m.toolInfo.Name) == "" {
		return nil, errors.New("invalid MCP tool information")
	}

	inputSchema := m.toolInfo.InputSchema
	if len(inputSchema) == 0 {
		inputSchema = json.RawMessage(`{"type":"object"}`)
	}

	var parameterSchema einojsonschema.Schema
	if err := json.Unmarshal(inputSchema, &parameterSchema); err != nil {
		return nil, fmt.Errorf("parse input schema of MCP tool %q: %w", m.toolInfo.Name, err)
	}
	if parameterSchema.Type == "" {
		parameterSchema.Type = "object"
	}
	if parameterSchema.Type != "object" {
		return nil, fmt.Errorf("input schema of MCP tool %q must be an object", m.toolInfo.Name)
	}

	return &schema.ToolInfo{
		Name:        strings.TrimSpace(m.toolInfo.Name),
		Desc:        m.toolInfo.Description,
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&parameterSchema),
	}, nil
}

func (m *mcpInvokableTool) InvokableRun(
	ctx context.Context,
	argumentsInJSON string,
	_ ...tool.Option,
) (string, error) {
	if m == nil || m.toolInfo == nil {
		return "", errors.New("invalid MCP tool")
	}

	service := crossmcp.DefaultSVC()
	if service == nil {
		return "", errors.New("MCP service is not initialized")
	}

	response, err := service.ExecuteTool(ctx, &model.ExecuteToolRequest{
		SpaceID:     m.spaceID,
		MCPServerID: m.toolInfo.MCPServerID,
		ToolName:    m.toolInfo.Name,
		Arguments:   json.RawMessage(argumentsInJSON),
	})
	if err != nil {
		return "", err
	}
	if response == nil {
		return "", errors.New("MCP service returned an empty response")
	}
	if response.IsError {
		return fmt.Sprintf("MCP tool error: %s", response.Output), nil
	}

	return response.Output, nil
}

func validateUniqueToolNames(ctx context.Context, tools []tool.BaseTool) error {
	toolNames := make(map[string]struct{}, len(tools))
	for _, agentTool := range tools {
		if agentTool == nil {
			return errors.New("agent contains an invalid tool")
		}

		toolInfo, err := agentTool.Info(ctx)
		if err != nil {
			return fmt.Errorf("get agent tool information: %w", err)
		}
		if toolInfo == nil || strings.TrimSpace(toolInfo.Name) == "" {
			return errors.New("agent contains a tool without a name")
		}

		toolName := strings.TrimSpace(toolInfo.Name)
		if _, exists := toolNames[toolName]; exists {
			return fmt.Errorf("duplicate agent tool name %q", toolName)
		}
		toolNames[toolName] = struct{}{}
	}

	return nil
}
