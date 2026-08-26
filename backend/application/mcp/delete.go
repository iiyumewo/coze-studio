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
	"errors"

	basemodel "github.com/coze-dev/coze-studio/backend/api/model/base"
	mcpmodel "github.com/coze-dev/coze-studio/backend/api/model/mcp"
	rescommon "github.com/coze-dev/coze-studio/backend/api/model/resource/common"
	searchentity "github.com/coze-dev/coze-studio/backend/domain/search/entity"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

func (s *MCPApplicationService) DeleteMCPServer(
	ctx context.Context,
	req *mcpmodel.DeleteMCPServerRequest,
) (*mcpmodel.DeleteMCPServerResponse, error) {
	if req == nil {
		return nil, errorx.New(errno.ErrMCPInvalidParamCode, errorx.KV("msg", "request is required"))
	}
	if req.GetMCPServerID() <= 0 {
		return nil, errorx.New(errno.ErrMCPInvalidParamCode,
			errorx.KV("msg", "mcp_server_id must be greater than zero"))
	}

	if _, err := authorizeWorkspaceAccess(ctx, req.GetSpaceID()); err != nil {
		return nil, err
	}

	server, err := s.mcpServerRepo.GetByID(ctx, req.GetMCPServerID())
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrMCPDeleteCode)
	}
	if server == nil {
		return nil, errorx.WrapByCode(errors.New("MCP server not found"), errno.ErrMCPDeleteCode)
	}
	if server.SpaceID != req.GetSpaceID() {
		return nil, errorx.New(errno.ErrMCPPermissionCode,
			errorx.KV("msg", "no permission for MCP server"))
	}

	if err = s.mcpServerRepo.Delete(ctx, req.GetMCPServerID(), req.GetSpaceID()); err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrMCPDeleteCode)
	}

	err = s.eventBus.PublishResources(ctx, &searchentity.ResourceDomainEvent{
		OpType: searchentity.Deleted,
		Resource: &searchentity.ResourceDocument{
			ResType: rescommon.ResType_MCP,
			ResID:   req.GetMCPServerID(),
		},
	})
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrMCPDeleteCode,
			errorx.KV("msg", "failed to publish MCP resource event"))
	}

	return &mcpmodel.DeleteMCPServerResponse{
		Code:     0,
		Msg:      "success",
		BaseResp: basemodel.NewBaseResp(),
	}, nil
}
