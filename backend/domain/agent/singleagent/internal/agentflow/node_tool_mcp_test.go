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
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"

	"github.com/coze-dev/coze-studio/backend/api/model/app/bot_common"
	crossmcp "github.com/coze-dev/coze-studio/backend/crossdomain/mcp"
	"github.com/coze-dev/coze-studio/backend/crossdomain/mcp/model"
)

type fakeMCPService struct {
	toolInfos      []*model.ToolInfo
	toolResponse   *model.ExecuteToolResponse
	getRequest     *model.MGetAgentToolsRequest
	executeRequest *model.ExecuteToolRequest
}

func (f *fakeMCPService) MGetAgentTools(
	_ context.Context,
	request *model.MGetAgentToolsRequest,
) ([]*model.ToolInfo, error) {
	f.getRequest = request
	return f.toolInfos, nil
}

func (f *fakeMCPService) ExecuteTool(
	_ context.Context,
	request *model.ExecuteToolRequest,
) (*model.ExecuteToolResponse, error) {
	f.executeRequest = request
	return f.toolResponse, nil
}

func TestMCPToolAdapter(t *testing.T) {
	service := &fakeMCPService{
		toolInfos: []*model.ToolInfo{{
			MCPServerID: 7,
			Name:        "lookup",
			Description: "Look up a record",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		}},
		toolResponse: &model.ExecuteToolResponse{Output: "found"},
	}
	setTestMCPService(t, service)

	tools, err := newMCPTools(context.Background(), &mcpToolConfig{
		spaceID:    100,
		serverRefs: []*bot_common.MCPInfo{{MCPServerID: 7}},
	})
	if err != nil {
		t.Fatalf("newMCPTools() error = %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("newMCPTools() returned %d tools, want 1", len(tools))
	}
	if service.getRequest.SpaceID != 100 || len(service.getRequest.MCPServerIDs) != 1 || service.getRequest.MCPServerIDs[0] != 7 {
		t.Fatalf("MGetAgentTools() request = %#v", service.getRequest)
	}

	toolInfo, err := tools[0].Info(context.Background())
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}
	if toolInfo.Name != "lookup" || toolInfo.Desc != "Look up a record" {
		t.Fatalf("Info() = %#v", toolInfo)
	}
	parameterSchema, err := toolInfo.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatalf("ToJSONSchema() error = %v", err)
	}
	serializedSchema, err := json.Marshal(parameterSchema)
	if err != nil {
		t.Fatalf("Marshal() schema error = %v", err)
	}
	if !strings.Contains(string(serializedSchema), `"query"`) {
		t.Fatalf("tool schema = %s, want query property", serializedSchema)
	}

	output, err := tools[0].InvokableRun(context.Background(), `{"query":"coze"}`)
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}
	if output != "found" {
		t.Fatalf("InvokableRun() output = %q, want found", output)
	}
	if service.executeRequest.SpaceID != 100 || service.executeRequest.MCPServerID != 7 || service.executeRequest.ToolName != "lookup" {
		t.Fatalf("ExecuteTool() request = %#v", service.executeRequest)
	}
	if string(service.executeRequest.Arguments) != `{"query":"coze"}` {
		t.Fatalf("ExecuteTool() arguments = %s", service.executeRequest.Arguments)
	}
}

func TestMCPToolAdapterReturnsMCPToolErrorToModel(t *testing.T) {
	service := &fakeMCPService{
		toolResponse: &model.ExecuteToolResponse{Output: "request rejected", IsError: true},
	}
	setTestMCPService(t, service)

	adapter := &mcpInvokableTool{
		spaceID: 100,
		toolInfo: &model.ToolInfo{
			MCPServerID: 7,
			Name:        "write",
		},
	}
	output, err := adapter.InvokableRun(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}
	if output != "MCP tool error: request rejected" {
		t.Fatalf("InvokableRun() output = %q", output)
	}
}

func TestValidateUniqueToolNamesRejectsCollision(t *testing.T) {
	tools := []tool.BaseTool{
		&mcpInvokableTool{toolInfo: &model.ToolInfo{Name: "lookup", InputSchema: json.RawMessage(`{"type":"object"}`)}},
		&mcpInvokableTool{toolInfo: &model.ToolInfo{Name: "lookup", InputSchema: json.RawMessage(`{"type":"object"}`)}},
	}

	if err := validateUniqueToolNames(context.Background(), tools); err == nil {
		t.Fatal("validateUniqueToolNames() error = nil, want duplicate name error")
	}
}

func setTestMCPService(t *testing.T, service crossmcp.MCPService) {
	t.Helper()
	previousService := crossmcp.DefaultSVC()
	crossmcp.SetDefaultSVC(service)
	t.Cleanup(func() {
		crossmcp.SetDefaultSVC(previousService)
	})
}
