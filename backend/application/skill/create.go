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
	"fmt"
	"path/filepath"
	"time"

	rescommon "github.com/coze-dev/coze-studio/backend/api/model/resource/common"
	"github.com/coze-dev/coze-studio/backend/domain/search/entity"
	skillentity "github.com/coze-dev/coze-studio/backend/domain/skill/entity"
	uploadentity "github.com/coze-dev/coze-studio/backend/domain/upload/entity"
	uploadservice "github.com/coze-dev/coze-studio/backend/domain/upload/service"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

func (s *SkillApplicationService) CreateSkill(ctx context.Context, spaceID int64, fileName string, data []byte) (*skillentity.Skill, error) {
	creatorID, err := authorizeWorkspaceAccess(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	metadata, err := validateSkillArchive(fileName, data)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrSkillValidateCode,
			errorx.KV("msg", errorx.ErrorWithoutStack(err)))
	}
	if s.storage == nil || s.fileSVC == nil {
		return nil, errorx.New(errno.ErrSkillCreateCode, errorx.KV("msg", "skill storage is not initialized"))
	}

	skillID, err := s.idGen.GenID(ctx)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrSkillCreateCode)
	}
	cleanName := filepath.Base(fileName)
	objectKey := fmt.Sprintf("skills/%d/%d/%s", spaceID, skillID, cleanName)
	if err = s.storage.PutObject(ctx, objectKey, data); err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrSkillCreateCode,
			errorx.KV("msg", "failed to store skill archive"))
	}

	now := timeNowUnixMilli()
	fileResp, err := s.fileSVC.UploadFile(ctx, &uploadservice.UploadFileRequest{File: &uploadentity.File{
		Name:          cleanName,
		FileSize:      int64(len(data)),
		TosURI:        objectKey,
		Status:        uploadentity.FileStatusValid,
		Source:        uploadentity.FileSourceAPI,
		CreatorID:     fmt.Sprintf("%d", creatorID),
		CozeAccountID: creatorID,
		ContentType:   "application/zip",
		CreatedAt:     now,
		UpdatedAt:     now,
	}})
	if err != nil {
		_ = s.storage.DeleteObject(ctx, objectKey)
		return nil, errorx.WrapByCode(err, errno.ErrSkillCreateCode,
			errorx.KV("msg", "failed to save skill file metadata"))
	}
	if fileResp == nil || fileResp.File == nil || fileResp.File.ID <= 0 {
		_ = s.storage.DeleteObject(ctx, objectKey)
		return nil, errorx.New(errno.ErrSkillCreateCode,
			errorx.KV("msg", "skill file metadata is empty"))
	}

	created, err := s.repo.Create(ctx, &skillentity.Skill{
		ID:          skillID,
		SpaceID:     spaceID,
		CreatorID:   creatorID,
		Name:        metadata.Name,
		Description: metadata.Description,
		FileID:      fileResp.File.ID,
		FileName:    cleanName,
		FileSize:    int64(len(data)),
		EntryCount:  metadata.EntryCount,
	})
	if err != nil {
		_ = s.fileSVC.DeleteFile(ctx, &uploadservice.DeleteFileRequest{ID: fileResp.File.ID})
		_ = s.storage.DeleteObject(ctx, objectKey)
		return nil, errorx.WrapByCode(err, errno.ErrSkillCreateCode)
	}

	if err = s.eventBus.PublishResources(ctx, &entity.ResourceDomainEvent{
		OpType: entity.Created,
		Resource: &entity.ResourceDocument{
			ResType:       rescommon.ResType_Skill,
			ResID:         created.ID,
			Name:          &created.Name,
			SpaceID:       &created.SpaceID,
			OwnerID:       &created.CreatorID,
			PublishStatus: ptr.Of(rescommon.PublishStatus_Published),
			CreateTimeMS:  &created.CreatedAt,
			UpdateTimeMS:  &created.UpdatedAt,
		},
	}); err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrSkillCreateCode,
			errorx.KV("msg", "failed to publish skill resource event"))
	}
	return created, nil
}

func timeNowUnixMilli() int64 { return time.Now().UnixMilli() }
