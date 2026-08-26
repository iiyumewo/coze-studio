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

package dal

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-studio/backend/domain/mcp/entity"
	"github.com/coze-dev/coze-studio/backend/domain/mcp/internal/dal/model"
	"github.com/coze-dev/coze-studio/backend/domain/mcp/internal/dal/query"
	"github.com/coze-dev/coze-studio/backend/infra/idgen"
)

type MCPServerDAO struct {
	idGen idgen.IDGenerator
	query *query.Query
}

func NewMCPServerDAO(db *gorm.DB, idGen idgen.IDGenerator) *MCPServerDAO {
	return &MCPServerDAO{
		idGen: idGen,
		query: query.Use(db),
	}
}

func (dao *MCPServerDAO) Create(ctx context.Context, server *entity.MCPServer) (*entity.MCPServer, error) {
	if server == nil {
		return nil, errors.New("MCP server is required")
	}

	id, err := dao.idGen.GenID(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UnixMilli()
	record := serverToModel(server)
	record.ID = id
	record.Status = int32(entity.ServerStatusEnabled)
	record.ValidationStatus = string(entity.ValidationStatusSuccess)
	record.LastError = ""
	record.LastCheckedAt = now
	record.CreatedAt = now
	record.UpdatedAt = now

	if record.Transport == "" {
		record.Transport = string(entity.TransportTypeStreamableHTTP)
	}
	if record.AuthType == "" {
		record.AuthType = string(entity.AuthenticationTypeNone)
	}

	if err = dao.query.McpServer.WithContext(ctx).Create(record); err != nil {
		return nil, err
	}

	return modelToServer(record), nil
}

func (dao *MCPServerDAO) GetByID(ctx context.Context, id int64) (*entity.MCPServer, error) {
	mcpServer := dao.query.McpServer
	record, err := mcpServer.WithContext(ctx).Where(mcpServer.ID.Eq(id)).First()
	if err != nil {
		return nil, err
	}

	return modelToServer(record), nil
}

func serverToModel(server *entity.MCPServer) *model.McpServer {
	return &model.McpServer{
		ID:              server.ID,
		SpaceID:         server.SpaceID,
		CreatorID:       server.CreatorID,
		Name:            server.Name,
		Description:     server.Description,
		ServerURL:       server.ServerURL,
		Transport:       string(server.Transport),
		AuthType:        string(server.AuthType),
		EncryptedAuth:   server.EncryptedAuth,
		Tools:           server.Tools,
		ProtocolVersion: server.ProtocolVersion,
		ServerVersion:   server.ServerVersion,
	}
}

func modelToServer(record *model.McpServer) *entity.MCPServer {
	return &entity.MCPServer{
		ID:               record.ID,
		SpaceID:          record.SpaceID,
		CreatorID:        record.CreatorID,
		Name:             record.Name,
		Description:      record.Description,
		ServerURL:        record.ServerURL,
		Transport:        entity.TransportType(record.Transport),
		AuthType:         entity.AuthenticationType(record.AuthType),
		EncryptedAuth:    record.EncryptedAuth,
		Tools:            record.Tools,
		ProtocolVersion:  record.ProtocolVersion,
		ServerVersion:    record.ServerVersion,
		Status:           entity.ServerStatus(record.Status),
		ValidationStatus: entity.ValidationStatus(record.ValidationStatus),
		LastError:        record.LastError,
		LastCheckedAt:    record.LastCheckedAt,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}
}
