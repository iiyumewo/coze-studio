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

package impl

import (
	"context"

	crossmcp "github.com/coze-dev/coze-studio/backend/crossdomain/mcp"
	"github.com/coze-dev/coze-studio/backend/crossdomain/mcp/model"
	mcpservice "github.com/coze-dev/coze-studio/backend/domain/mcp/service"
)

type mcpServiceImpl struct {
	domainSVC mcpservice.MCPService
}

func InitDomainService(domainSVC mcpservice.MCPService) crossmcp.MCPService {
	return &mcpServiceImpl{domainSVC: domainSVC}
}

func (s *mcpServiceImpl) MGetAgentTools(
	ctx context.Context,
	req *model.MGetAgentToolsRequest,
) ([]*model.ToolInfo, error) {
	return s.domainSVC.MGetAgentTools(ctx, req)
}

func (s *mcpServiceImpl) ExecuteTool(
	ctx context.Context,
	req *model.ExecuteToolRequest,
) (*model.ExecuteToolResponse, error) {
	return s.domainSVC.ExecuteTool(ctx, req)
}
