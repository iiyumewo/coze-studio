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
	"time"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-studio/backend/domain/skill/entity"
	"github.com/coze-dev/coze-studio/backend/infra/idgen"
)

type SkillDAO struct {
	db    *gorm.DB
	idGen idgen.IDGenerator
}

func NewSkillDAO(db *gorm.DB, idGen idgen.IDGenerator) *SkillDAO {
	return &SkillDAO{db: db, idGen: idGen}
}

type skillPO struct {
	ID          int64          `gorm:"column:id;primaryKey"`
	SpaceID     int64          `gorm:"column:space_id"`
	CreatorID   int64          `gorm:"column:creator_id"`
	Name        string         `gorm:"column:name"`
	Description string         `gorm:"column:description"`
	FileID      int64          `gorm:"column:file_id"`
	FileName    string         `gorm:"column:file_name"`
	FileSize    int64          `gorm:"column:file_size"`
	EntryCount  int32          `gorm:"column:entry_count"`
	CreatedAt   int64          `gorm:"column:created_at"`
	UpdatedAt   int64          `gorm:"column:updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (skillPO) TableName() string { return "skill" }

func (d *SkillDAO) Create(ctx context.Context, skill *entity.Skill) (*entity.Skill, error) {
	if skill == nil {
		return nil, gorm.ErrInvalidData
	}
	id := skill.ID
	if id == 0 {
		var err error
		id, err = d.idGen.GenID(ctx)
		if err != nil {
			return nil, err
		}
	}
	now := time.Now().UnixMilli()
	po := skillToPO(skill)
	po.ID = id
	po.CreatedAt = now
	po.UpdatedAt = now
	if err := d.db.WithContext(ctx).Create(po).Error; err != nil {
		return nil, err
	}
	return poToSkill(po), nil
}

func (d *SkillDAO) GetByID(ctx context.Context, id int64) (*entity.Skill, error) {
	var po skillPO
	if err := d.db.WithContext(ctx).Where("id = ?", id).First(&po).Error; err != nil {
		return nil, err
	}
	return poToSkill(&po), nil
}

func (d *SkillDAO) Delete(ctx context.Context, id, spaceID int64) error {
	result := d.db.WithContext(ctx).Where("id = ? AND space_id = ?", id, spaceID).Delete(&skillPO{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func skillToPO(skill *entity.Skill) *skillPO {
	return &skillPO{
		ID:          skill.ID,
		SpaceID:     skill.SpaceID,
		CreatorID:   skill.CreatorID,
		Name:        skill.Name,
		Description: skill.Description,
		FileID:      skill.FileID,
		FileName:    skill.FileName,
		FileSize:    skill.FileSize,
		EntryCount:  skill.EntryCount,
		CreatedAt:   skill.CreatedAt,
		UpdatedAt:   skill.UpdatedAt,
	}
}

func poToSkill(po *skillPO) *entity.Skill {
	return &entity.Skill{
		ID:          po.ID,
		SpaceID:     po.SpaceID,
		CreatorID:   po.CreatorID,
		Name:        po.Name,
		Description: po.Description,
		FileID:      po.FileID,
		FileName:    po.FileName,
		FileSize:    po.FileSize,
		EntryCount:  po.EntryCount,
		CreatedAt:   po.CreatedAt,
		UpdatedAt:   po.UpdatedAt,
	}
}
