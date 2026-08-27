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

package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-studio/backend/domain/mcp/entity"
	"github.com/coze-dev/coze-studio/backend/domain/mcp/internal/dal"
	"github.com/coze-dev/coze-studio/backend/infra/idgen"
)

func NewMCPServerRepo(db *gorm.DB, idGen idgen.IDGenerator) MCPServerRepository {
	return dal.NewMCPServerDAO(db, idGen)
}

func NewMCPServerReadRepo(db *gorm.DB, idGen idgen.IDGenerator) MCPServerReadRepository {
	return dal.NewMCPServerDAO(db, idGen)
}

type MCPServerRepository interface {
	Create(ctx context.Context, server *entity.MCPServer) (*entity.MCPServer, error)
	GetByID(ctx context.Context, id int64) (*entity.MCPServer, error)
	Delete(ctx context.Context, id, spaceID int64) error
}

type MCPServerReadRepository interface {
	GetByID(ctx context.Context, id int64) (*entity.MCPServer, error)
	MGetByIDs(ctx context.Context, spaceID int64, ids []int64) ([]*entity.MCPServer, error)
}
