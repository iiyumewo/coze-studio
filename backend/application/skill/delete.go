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
	"errors"
	"fmt"
	"path/filepath"

	"gorm.io/gorm"

	rescommon "github.com/coze-dev/coze-studio/backend/api/model/resource/common"
	searchentity "github.com/coze-dev/coze-studio/backend/domain/search/entity"
	uploadservice "github.com/coze-dev/coze-studio/backend/domain/upload/service"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

func (s *SkillApplicationService) DeleteSkill(ctx context.Context, id, spaceID int64) error {
	if _, err := authorizeWorkspaceAccess(ctx, spaceID); err != nil {
		return err
	}
	if id <= 0 {
		return errorx.New(errno.ErrSkillInvalidParamCode, errorx.KV("msg", "skill_id must be greater than zero"))
	}

	sk, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.New(errno.ErrSkillGetCode, errorx.KV("msg", "Skill not found"))
		}
		return errorx.WrapByCode(err, errno.ErrSkillGetCode)
	}
	if sk == nil || sk.SpaceID != spaceID {
		return errorx.New(errno.ErrSkillPermissionCode, errorx.KV("msg", "no permission for Skill"))
	}

	if err := s.repo.Delete(ctx, id, spaceID); err != nil {
		return errorx.WrapByCode(err, errno.ErrSkillDeleteCode)
	}

	// The database row is the source of truth. Cleanup of the associated file is
	// best effort so a transient object-storage failure does not leave a stale
	// resource in the search index.
	if s.fileSVC != nil && sk.FileID > 0 {
		if err := s.fileSVC.DeleteFile(ctx, &uploadservice.DeleteFileRequest{ID: sk.FileID}); err != nil {
			logs.CtxWarnf(ctx, "delete Skill file metadata failed, skill_id=%d, file_id=%d, err=%v", id, sk.FileID, err)
		}
	}
	if s.storage != nil {
		objectKey := fmt.Sprintf("skills/%d/%d/%s", spaceID, id, filepath.Base(sk.FileName))
		if err := s.storage.DeleteObject(ctx, objectKey); err != nil {
			logs.CtxWarnf(ctx, "delete Skill archive failed, skill_id=%d, object_key=%s, err=%v", id, objectKey, err)
		}
	}

	if s.eventBus == nil {
		return nil
	}
	if err := s.eventBus.PublishResources(ctx, &searchentity.ResourceDomainEvent{
		OpType: searchentity.Deleted,
		Resource: &searchentity.ResourceDocument{
			ResType: rescommon.ResType_Skill,
			ResID:   id,
			SpaceID: ptr.Of(spaceID),
		},
	}); err != nil {
		return errorx.WrapByCode(err, errno.ErrSkillDeleteCode,
			errorx.KV("msg", "failed to publish Skill resource event"))
	}
	return nil
}
