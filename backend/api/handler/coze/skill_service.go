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

package coze

import (
	"context"
	"io"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	skillmodel "github.com/coze-dev/coze-studio/backend/api/model/skill"
	skillapplication "github.com/coze-dev/coze-studio/backend/application/skill"
)

const maxSkillUploadSize int64 = 20 * 1024 * 1024

// CreateSkill uploads and validates a Skill archive for the current workspace.
// @router /api/skill_manage/v1/skills [POST]
func CreateSkill(ctx context.Context, c *app.RequestContext) {
	spaceID, err := strconv.ParseInt(string(c.FormValue("space_id")), 10, 64)
	if err != nil || spaceID <= 0 {
		invalidParamRequestResponse(c, "space_id is invalid")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		invalidParamRequestResponse(c, "skill archive file is required")
		return
	}
	if fileHeader.Size > maxSkillUploadSize {
		invalidParamRequestResponse(c, "skill archive exceeds the 20 MB limit")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxSkillUploadSize+1))
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}
	if int64(len(data)) > maxSkillUploadSize {
		invalidParamRequestResponse(c, "skill archive exceeds the 20 MB limit")
		return
	}

	created, err := skillapplication.SkillApplicationSVC.CreateSkill(ctx, spaceID, fileHeader.Filename, data)
	if err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	c.JSON(consts.StatusOK, &skillmodel.CreateSkillResponse{
		Code: 0,
		Msg:  "success",
		Data: &skillmodel.SkillData{
			ID:          created.ID,
			Name:        created.Name,
			Description: created.Description,
			FileName:    created.FileName,
			FileSize:    created.FileSize,
			EntryCount:  created.EntryCount,
		},
	})
}

// DeleteSkill removes a Skill and its uploaded archive from the workspace.
// @router /api/skill_manage/v1/skills/:skill_id [DELETE]
func DeleteSkill(ctx context.Context, c *app.RequestContext) {
	skillID, err := strconv.ParseInt(string(c.Param("skill_id")), 10, 64)
	if err != nil || skillID <= 0 {
		invalidParamRequestResponse(c, "skill_id is invalid")
		return
	}
	spaceID, err := strconv.ParseInt(string(c.Query("space_id")), 10, 64)
	if err != nil || spaceID <= 0 {
		invalidParamRequestResponse(c, "space_id is invalid")
		return
	}

	if err := skillapplication.SkillApplicationSVC.DeleteSkill(ctx, skillID, spaceID); err != nil {
		internalServerErrorResponse(ctx, c, err)
		return
	}

	c.JSON(consts.StatusOK, &skillmodel.DeleteSkillResponse{Code: 0, Msg: "success"})
}
