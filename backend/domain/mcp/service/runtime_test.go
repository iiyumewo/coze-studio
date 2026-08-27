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
	"encoding/json"
	"errors"
	"testing"

	crossmodel "github.com/coze-dev/coze-studio/backend/crossdomain/mcp/model"
	"github.com/coze-dev/coze-studio/backend/domain/mcp/entity"
	mcpinfra "github.com/coze-dev/coze-studio/backend/infra/mcp"
)

type fakeServerRepository struct {
	server  *entity.MCPServer
	servers []*entity.MCPServer
	err     error
}

func (f *fakeServerRepository) GetByID(context.Context, int64) (*entity.MCPServer, error) {
	return f.server, f.err
}

func (f *fakeServerRepository) MGetByIDs(context.Context, int64, []int64) ([]*entity.MCPServer, error) {
	return f.servers, f.err
}

type fakeToolClient struct {
	request *mcpinfra.CallToolRequest
	result  *mcpinfra.CallToolResult
	err     error
}

func (f *fakeToolClient) CallTool(_ context.Context, request *mcpinfra.CallToolRequest) (*mcpinfra.CallToolResult, error) {
	f.request = request
	return f.result, f.err
}

func TestMGetAgentToolsPreservesConfiguredServerOrder(t *testing.T) {
	service := NewService(&Components{
		ServerRepo: &fakeServerRepository{servers: []*entity.MCPServer{
			newRuntimeServer(2, 100, "second", "tool_b"),
			newRuntimeServer(1, 100, "first", "tool_a"),
		}},
	})

	tools, err := service.MGetAgentTools(context.Background(), &crossmodel.MGetAgentToolsRequest{
		SpaceID:      100,
		MCPServerIDs: []int64{1, 2},
	})
	if err != nil {
		t.Fatalf("MGetAgentTools() error = %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("MGetAgentTools() returned %d tools, want 2", len(tools))
	}
	if tools[0].MCPServerID != 1 || tools[0].Name != "tool_a" {
		t.Fatalf("first tool = %#v, want tool_a from server 1", tools[0])
	}
	if tools[1].MCPServerID != 2 || tools[1].Name != "tool_b" {
		t.Fatalf("second tool = %#v, want tool_b from server 2", tools[1])
	}
}

func TestMGetAgentToolsRejectsUnavailableServer(t *testing.T) {
	server := newRuntimeServer(1, 100, "server", "lookup")
	server.Status = entity.ServerStatusDisabled
	service := NewService(&Components{
		ServerRepo: &fakeServerRepository{servers: []*entity.MCPServer{server}},
	})

	_, err := service.MGetAgentTools(context.Background(), &crossmodel.MGetAgentToolsRequest{
		SpaceID:      100,
		MCPServerIDs: []int64{1},
	})
	if err == nil {
		t.Fatal("MGetAgentTools() error = nil, want disabled server error")
	}
}

func TestExecuteToolDecryptsAuthenticationAndCallsClient(t *testing.T) {
	encryptedAuthentication, err := mcpinfra.EncryptAuthentication(mcpinfra.Authentication{
		Type:        mcpinfra.AuthenticationTypeHeader,
		HeaderName:  "Authorization",
		HeaderValue: "Bearer test-secret",
	})
	if err != nil {
		t.Fatalf("EncryptAuthentication() error = %v", err)
	}

	server := newRuntimeServer(1, 100, "server", "lookup")
	server.AuthType = entity.AuthenticationTypeHeader
	server.EncryptedAuth = encryptedAuthentication
	toolClient := &fakeToolClient{result: &mcpinfra.CallToolResult{Output: "found", IsError: false}}
	service := NewService(&Components{
		ServerRepo: &fakeServerRepository{server: server},
		ToolClient: toolClient,
	})

	response, err := service.ExecuteTool(context.Background(), &crossmodel.ExecuteToolRequest{
		SpaceID:     100,
		MCPServerID: 1,
		ToolName:    "lookup",
		Arguments:   json.RawMessage(`{"key":"coze"}`),
	})
	if err != nil {
		t.Fatalf("ExecuteTool() error = %v", err)
	}
	if response.Output != "found" || response.IsError {
		t.Fatalf("ExecuteTool() response = %#v", response)
	}
	if toolClient.request == nil {
		t.Fatal("MCP tool client was not called")
	}
	if toolClient.request.Authentication.HeaderName != "Authorization" ||
		toolClient.request.Authentication.HeaderValue != "Bearer test-secret" {
		t.Fatalf("CallTool() authentication = %#v", toolClient.request.Authentication)
	}
	if string(toolClient.request.Arguments) != `{"key":"coze"}` {
		t.Fatalf("CallTool() arguments = %s", toolClient.request.Arguments)
	}
}

func TestExecuteToolRejectsUndeclaredTool(t *testing.T) {
	toolClient := &fakeToolClient{err: errors.New("must not be called")}
	service := NewService(&Components{
		ServerRepo: &fakeServerRepository{server: newRuntimeServer(1, 100, "server", "lookup")},
		ToolClient: toolClient,
	})

	_, err := service.ExecuteTool(context.Background(), &crossmodel.ExecuteToolRequest{
		SpaceID:     100,
		MCPServerID: 1,
		ToolName:    "delete_everything",
		Arguments:   json.RawMessage(`{}`),
	})
	if err == nil {
		t.Fatal("ExecuteTool() error = nil, want undeclared tool error")
	}
	if toolClient.request != nil {
		t.Fatal("MCP tool client was called for an undeclared tool")
	}
}

func TestExecuteToolRejectsServerFromAnotherWorkspace(t *testing.T) {
	toolClient := &fakeToolClient{err: errors.New("must not be called")}
	service := NewService(&Components{
		ServerRepo: &fakeServerRepository{server: newRuntimeServer(1, 200, "server", "lookup")},
		ToolClient: toolClient,
	})

	_, err := service.ExecuteTool(context.Background(), &crossmodel.ExecuteToolRequest{
		SpaceID:     100,
		MCPServerID: 1,
		ToolName:    "lookup",
		Arguments:   json.RawMessage(`{}`),
	})
	if err == nil {
		t.Fatal("ExecuteTool() error = nil, want workspace isolation error")
	}
	if toolClient.request != nil {
		t.Fatal("MCP tool client was called across workspaces")
	}
}

func newRuntimeServer(id, spaceID int64, name, toolName string) *entity.MCPServer {
	return &entity.MCPServer{
		ID:               id,
		SpaceID:          spaceID,
		Name:             name,
		ServerURL:        "https://example.com/mcp",
		Transport:        entity.TransportTypeStreamableHTTP,
		AuthType:         entity.AuthenticationTypeNone,
		Status:           entity.ServerStatusEnabled,
		ValidationStatus: entity.ValidationStatusSuccess,
		Tools: []*entity.Tool{{
			Name:        toolName,
			Description: "test tool",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
	}
}
