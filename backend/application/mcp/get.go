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
	mcpentity "github.com/coze-dev/coze-studio/backend/domain/mcp/entity"
	mcpinfra "github.com/coze-dev/coze-studio/backend/infra/mcp"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

func (s *MCPApplicationService) GetMCPServer(
	ctx context.Context,
	req *mcpmodel.GetMCPServerRequest,
) (*mcpmodel.GetMCPServerResponse, error) {
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
		return nil, errorx.WrapByCode(err, errno.ErrMCPGetCode)
	}
	if server == nil {
		return nil, errorx.WrapByCode(errors.New("MCP server not found"), errno.ErrMCPGetCode)
	}
	if server.SpaceID != req.GetSpaceID() {
		return nil, errorx.New(errno.ErrMCPPermissionCode,
			errorx.KV("msg", "no permission for MCP server"))
	}

	detail, err := convertMCPServerDetail(server)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrMCPGetCode)
	}

	return &mcpmodel.GetMCPServerResponse{
		Data:     detail,
		Code:     0,
		Msg:      "success",
		BaseResp: basemodel.NewBaseResp(),
	}, nil
}

func convertMCPServerDetail(server *mcpentity.MCPServer) (*mcpmodel.MCPServerDetailData, error) {
	authentication, err := convertMCPAuthenticationDetail(server)
	if err != nil {
		return nil, err
	}

	tools := make([]*mcpmodel.MCPToolData, 0, len(server.Tools))
	for _, tool := range server.Tools {
		if tool == nil {
			continue
		}

		tools = append(tools, &mcpmodel.MCPToolData{
			Name:        tool.Name,
			Title:       optionalString(tool.Title),
			Description: optionalString(tool.Description),
		})
	}

	return &mcpmodel.MCPServerDetailData{
		ID:               server.ID,
		Name:             server.Name,
		Description:      optionalString(server.Description),
		ServerURL:        server.ServerURL,
		Transport:        string(server.Transport),
		Authentication:   authentication,
		Tools:            tools,
		ProtocolVersion:  optionalString(server.ProtocolVersion),
		ServerVersion:    optionalString(server.ServerVersion),
		Status:           int32(server.Status),
		ValidationStatus: string(server.ValidationStatus),
		LastError:        optionalString(server.LastError),
		LastCheckedAt:    server.LastCheckedAt,
		CreatedAt:        server.CreatedAt,
		UpdatedAt:        server.UpdatedAt,
	}, nil
}

func convertMCPAuthenticationDetail(
	server *mcpentity.MCPServer,
) (*mcpmodel.MCPServerAuthenticationData, error) {
	authentication := &mcpmodel.MCPServerAuthenticationData{Type: string(server.AuthType)}
	decryptedAuthentication, err := mcpinfra.DecryptAuthentication(
		mcpinfra.AuthenticationType(server.AuthType),
		server.EncryptedAuth,
	)
	if err != nil {
		return nil, err
	}
	if decryptedAuthentication.Type == mcpinfra.AuthenticationTypeHeader {
		authentication.HeaderName = ptr.Of(decryptedAuthentication.HeaderName)
	}

	return authentication, nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}

	return ptr.Of(value)
}
