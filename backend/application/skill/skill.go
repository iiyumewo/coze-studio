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

package skill

import (
	"context"

	"gorm.io/gorm"

	searchservice "github.com/coze-dev/coze-studio/backend/domain/search/service"
	"github.com/coze-dev/coze-studio/backend/domain/skill/entity"
	"github.com/coze-dev/coze-studio/backend/domain/skill/repository"
	uploadservice "github.com/coze-dev/coze-studio/backend/domain/upload/service"
	"github.com/coze-dev/coze-studio/backend/infra/idgen"
	"github.com/coze-dev/coze-studio/backend/infra/storage"
)

type SkillApplicationService struct {
	repo     repository.SkillRepository
	idGen    idgen.IDGenerator
	storage  storage.Storage
	fileSVC  uploadservice.UploadService
	eventBus searchservice.ResourceEventBus
}

var SkillApplicationSVC *SkillApplicationService

func InitService(db *gorm.DB, idGen idgen.IDGenerator, objectStorage storage.Storage, fileSVC uploadservice.UploadService, eventBus searchservice.ResourceEventBus) *SkillApplicationService {
	SkillApplicationSVC = &SkillApplicationService{
		repo:     repository.NewSkillRepo(db, idGen),
		idGen:    idGen,
		storage:  objectStorage,
		fileSVC:  fileSVC,
		eventBus: eventBus,
	}
	return SkillApplicationSVC
}

func (s *SkillApplicationService) GetByID(ctx context.Context, id int64) (*entity.Skill, error) {
	return s.repo.GetByID(ctx, id)
}
