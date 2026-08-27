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

	crossmodel "github.com/coze-dev/coze-studio/backend/crossdomain/mcp/model"
	"github.com/coze-dev/coze-studio/backend/domain/mcp/repository"
	mcpinfra "github.com/coze-dev/coze-studio/backend/infra/mcp"
)

type MCPService interface {
	MGetAgentTools(ctx context.Context, req *crossmodel.MGetAgentToolsRequest) ([]*crossmodel.ToolInfo, error)
	ExecuteTool(ctx context.Context, req *crossmodel.ExecuteToolRequest) (*crossmodel.ExecuteToolResponse, error)
}

type Components struct {
	ServerRepo repository.MCPServerReadRepository
	ToolClient mcpinfra.ToolClient
}

func NewService(components *Components) MCPService {
	return &mcpServiceImpl{components: components}
}

type mcpServiceImpl struct {
	components *Components
}
