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
	"gorm.io/gorm"

	"github.com/coze-dev/coze-studio/backend/application/search"
	"github.com/coze-dev/coze-studio/backend/domain/mcp/repository"
	mcpservice "github.com/coze-dev/coze-studio/backend/domain/mcp/service"
	"github.com/coze-dev/coze-studio/backend/infra/idgen"
	mcpinfra "github.com/coze-dev/coze-studio/backend/infra/mcp"
)

func InitService(db *gorm.DB, idGen idgen.IDGenerator, eventBus search.ResourceEventBus) *MCPApplicationService {
	MCPApplicationSVC = newApplicationService(
		mcpinfra.NewClient(),
		repository.NewMCPServerRepo(db, idGen),
		eventBus,
	)
	MCPApplicationSVC.DomainSVC = mcpservice.NewService(&mcpservice.Components{
		ServerRepo: repository.NewMCPServerReadRepo(db, idGen),
		ToolClient: mcpinfra.NewToolClient(),
	})

	return MCPApplicationSVC
}
