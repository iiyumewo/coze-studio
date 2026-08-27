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
	"strings"

	basemodel "github.com/coze-dev/coze-studio/backend/api/model/base"
	mcpmodel "github.com/coze-dev/coze-studio/backend/api/model/mcp"
	rescommon "github.com/coze-dev/coze-studio/backend/api/model/resource/common"
	"github.com/coze-dev/coze-studio/backend/application/base/ctxutil"
	crossuser "github.com/coze-dev/coze-studio/backend/crossdomain/user"
	mcpentity "github.com/coze-dev/coze-studio/backend/domain/mcp/entity"
	searchentity "github.com/coze-dev/coze-studio/backend/domain/search/entity"
	mcpinfra "github.com/coze-dev/coze-studio/backend/infra/mcp"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

const (
	maxMCPServerNameLength        = 100
	maxMCPServerDescriptionLength = 2000
	maxMCPProtocolVersionLength   = 32
	maxMCPServerVersionLength     = 128
)

type testedMCPServer struct {
	spaceID       int64
	creatorID     int64
	name          string
	serverURL     string
	transport     mcpentity.TransportType
	authType      mcpentity.AuthenticationType
	encryptedAuth *string
	testResult    *mcpinfra.TestResult
}

type headerAuthenticationPayload = mcpinfra.HeaderAuthenticationPayload

func (s *MCPApplicationService) TestAndCreateMCPServer(
	ctx context.Context,
	req *mcpmodel.TestAndCreateMCPServerRequest,
) (*mcpmodel.TestAndCreateMCPServerResponse, error) {
	testedServer, err := s.testMCPServer(ctx, req)
	if err != nil {
		return nil, err
	}

	createdServer, err := s.createMCPServer(ctx, testedServer)
	if err != nil {
		return nil, err
	}

	return &mcpmodel.TestAndCreateMCPServerResponse{
		Data: &mcpmodel.MCPServerData{
			ID:        createdServer.ID,
			Name:      createdServer.Name,
			ToolCount: int32(len(createdServer.Tools)),
		},
		Code:     0,
		Msg:      "success",
		BaseResp: basemodel.NewBaseResp(),
	}, nil
}

func (s *MCPApplicationService) testMCPServer(
	ctx context.Context,
	req *mcpmodel.TestAndCreateMCPServerRequest,
) (*testedMCPServer, error) {
	if req == nil {
		return nil, errorx.New(errno.ErrMCPInvalidParamCode, errorx.KV("msg", "request is required"))
	}

	transport := mcpentity.TransportType(strings.TrimSpace(req.GetTransport()))
	if transport != mcpentity.TransportTypeStreamableHTTP {
		return nil, errorx.New(errno.ErrMCPInvalidParamCode,
			errorx.KV("msg", "transport must be streamable_http"))
	}

	authentication := req.GetAuthentication()
	if authentication == nil {
		return nil, errorx.New(errno.ErrMCPInvalidParamCode,
			errorx.KV("msg", "authentication is required"))
	}

	creatorID, err := authorizeWorkspaceAccess(ctx, req.GetSpaceID())
	if err != nil {
		return nil, err
	}

	testAuthentication := mcpinfra.Authentication{
		Type:        mcpinfra.AuthenticationType(strings.TrimSpace(authentication.GetType())),
		HeaderName:  strings.TrimSpace(authentication.GetHeaderName()),
		HeaderValue: authentication.GetHeaderValue(),
	}
	serverURL := strings.TrimSpace(req.GetServerURL())
	testResult, err := s.mcpClient.Test(ctx, &mcpinfra.TestRequest{
		ServerURL:      serverURL,
		Authentication: testAuthentication,
	})
	if err != nil {
		if errors.Is(err, mcpinfra.ErrInvalidRequest) {
			return nil, errorx.WrapByCode(err, errno.ErrMCPInvalidParamCode,
				errorx.KV("msg", errorx.ErrorWithoutStack(err)))
		}

		return nil, errorx.WrapByCode(err, errno.ErrMCPValidateCode,
			errorx.KV("msg", errorx.ErrorWithoutStack(err)))
	}
	if testResult == nil {
		return nil, errorx.New(errno.ErrMCPValidateCode,
			errorx.KV("msg", "MCP server returned an empty test result"))
	}

	authType, encryptedAuth, err := encryptAuthentication(testAuthentication)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.GetName())
	if name == "" {
		name = strings.TrimSpace(testResult.ServerTitle)
	}
	if name == "" {
		name = strings.TrimSpace(testResult.ServerName)
	}
	if name == "" {
		name = "MCP Server"
	}
	if len([]rune(name)) > maxMCPServerNameLength {
		return nil, errorx.New(errno.ErrMCPInvalidParamCode,
			errorx.KV("msg", "name must not exceed 100 characters"))
	}

	return &testedMCPServer{
		spaceID:       req.GetSpaceID(),
		creatorID:     creatorID,
		name:          name,
		serverURL:     serverURL,
		transport:     transport,
		authType:      authType,
		encryptedAuth: encryptedAuth,
		testResult:    testResult,
	}, nil
}

func (s *MCPApplicationService) createMCPServer(
	ctx context.Context,
	testedServer *testedMCPServer,
) (*mcpentity.MCPServer, error) {
	if testedServer == nil || testedServer.testResult == nil {
		return nil, errorx.New(errno.ErrMCPCreateCode)
	}

	createdServer, err := s.mcpServerRepo.Create(ctx, &mcpentity.MCPServer{
		SpaceID:         testedServer.spaceID,
		CreatorID:       testedServer.creatorID,
		Name:            testedServer.name,
		Description:     truncateRunes(strings.TrimSpace(testedServer.testResult.Instructions), maxMCPServerDescriptionLength),
		ServerURL:       testedServer.serverURL,
		Transport:       testedServer.transport,
		AuthType:        testedServer.authType,
		EncryptedAuth:   testedServer.encryptedAuth,
		Tools:           convertTools(testedServer.testResult.Tools),
		ProtocolVersion: truncateRunes(strings.TrimSpace(testedServer.testResult.ProtocolVersion), maxMCPProtocolVersionLength),
		ServerVersion:   truncateRunes(strings.TrimSpace(testedServer.testResult.ServerVersion), maxMCPServerVersionLength),
	})
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrMCPCreateCode)
	}
	if createdServer == nil {
		return nil, errorx.New(errno.ErrMCPCreateCode)
	}

	err = s.eventBus.PublishResources(ctx, &searchentity.ResourceDomainEvent{
		OpType: searchentity.Created,
		Resource: &searchentity.ResourceDocument{
			ResType:       rescommon.ResType_MCP,
			ResID:         createdServer.ID,
			Name:          ptr.Of(createdServer.Name),
			SpaceID:       ptr.Of(createdServer.SpaceID),
			OwnerID:       ptr.Of(createdServer.CreatorID),
			PublishStatus: ptr.Of(rescommon.PublishStatus_Published),
			CreateTimeMS:  ptr.Of(createdServer.CreatedAt),
			UpdateTimeMS:  ptr.Of(createdServer.UpdatedAt),
		},
	})
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrMCPCreateCode,
			errorx.KV("msg", "failed to publish MCP resource event"))
	}

	return createdServer, nil
}

func convertTools(tools []*mcpinfra.Tool) []*mcpentity.Tool {
	convertedTools := make([]*mcpentity.Tool, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}

		convertedTool := &mcpentity.Tool{
			Name:         tool.Name,
			Title:        tool.Title,
			Description:  tool.Description,
			InputSchema:  append([]byte(nil), tool.InputSchema...),
			OutputSchema: append([]byte(nil), tool.OutputSchema...),
		}
		if tool.Annotations != nil {
			convertedTool.Annotations = &mcpentity.ToolAnnotations{
				Title:           tool.Annotations.Title,
				ReadOnlyHint:    tool.Annotations.ReadOnlyHint,
				DestructiveHint: cloneBool(tool.Annotations.DestructiveHint),
				IdempotentHint:  tool.Annotations.IdempotentHint,
				OpenWorldHint:   cloneBool(tool.Annotations.OpenWorldHint),
			}
		}

		convertedTools = append(convertedTools, convertedTool)
	}

	return convertedTools
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}

	clonedValue := *value
	return &clonedValue
}

func truncateRunes(value string, maxLength int) string {
	runes := []rune(value)
	if len(runes) <= maxLength {
		return value
	}

	return string(runes[:maxLength])
}

func encryptAuthentication(authentication mcpinfra.Authentication) (mcpentity.AuthenticationType, *string, error) {
	authType := mcpentity.AuthenticationType(authentication.Type)
	if authType == mcpentity.AuthenticationTypeNone {
		return authType, nil, nil
	}
	if authType != mcpentity.AuthenticationTypeHeader {
		return "", nil, errorx.New(errno.ErrMCPInvalidParamCode,
			errorx.KV("msg", "unsupported authentication type"))
	}

	encryptedAuth, err := mcpinfra.EncryptAuthentication(authentication)
	if err != nil {
		return "", nil, errorx.WrapByCode(err, errno.ErrMCPCreateCode)
	}

	return authType, encryptedAuth, nil
}

func authorizeWorkspaceAccess(ctx context.Context, spaceID int64) (int64, error) {
	if spaceID <= 0 {
		return 0, errorx.New(errno.ErrMCPInvalidParamCode, errorx.KV("msg", "space_id must be greater than zero"))
	}

	session := ctxutil.GetUserSessionFromCtx(ctx)
	if session == nil || session.UserID <= 0 {
		return 0, errorx.New(errno.ErrMCPPermissionCode, errorx.KV("msg", "session is required"))
	}

	spaces, err := crossuser.DefaultSVC().GetUserSpaceList(ctx, session.UserID)
	if err != nil {
		return 0, errorx.WrapByCode(err, errno.ErrMCPPermissionCode,
			errorx.KV("msg", "failed to verify workspace access"))
	}

	for _, space := range spaces {
		if space != nil && space.ID == spaceID {
			return session.UserID, nil
		}
	}

	return 0, errorx.New(errno.ErrMCPPermissionCode, errorx.KV("msg", "no permission for workspace"))
}
