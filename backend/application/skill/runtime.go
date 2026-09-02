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

	"github.com/coze-dev/coze-studio/backend/api/model/app/bot_common"
	skillentity "github.com/coze-dev/coze-studio/backend/domain/skill/entity"
	infraskill "github.com/coze-dev/coze-studio/backend/infra/skill"
)

// PrepareRuntime resolves the selected Workspace Skills and makes their archives
// available to AgentFlow without exposing storage details to the execution layer.
func (s *SkillApplicationService) PrepareRuntime(ctx context.Context, spaceID int64, refs []*bot_common.SkillInfo) (*infraskill.Catalog, error) {
	if len(refs) == 0 {
		return infraskill.PrepareRuntimeCatalog(ctx, nil, infraskill.DefaultRuntimeRoot(), spaceID, nil)
	}
	if s.repo == nil {
		return nil, fmt.Errorf("Skill repository is not initialized")
	}

	runtimeSkills := make([]infraskill.RuntimeSkill, 0, len(refs))
	seen := make(map[int64]struct{}, len(refs))
	for _, ref := range refs {
		if ref == nil || ref.GetSkillID() <= 0 {
			return nil, fmt.Errorf("invalid Skill reference")
		}
		skillID := ref.GetSkillID()
		if _, exists := seen[skillID]; exists {
			return nil, fmt.Errorf("duplicate Skill reference: %d", skillID)
		}
		seen[skillID] = struct{}{}

		sk, err := s.repo.GetByID(ctx, skillID)
		if err != nil {
			return nil, fmt.Errorf("get Skill %d: %w", skillID, err)
		}
		if sk == nil {
			return nil, fmt.Errorf("Skill %d was not found", skillID)
		}
		if sk.SpaceID != spaceID {
			return nil, fmt.Errorf("Skill %d does not belong to Workspace %d", skillID, spaceID)
		}
		runtimeSkills = append(runtimeSkills, toRuntimeSkill(sk))
	}

	return infraskill.PrepareRuntimeCatalog(ctx, s.storage, infraskill.DefaultRuntimeRoot(), spaceID, runtimeSkills)
}

func toRuntimeSkill(sk *skillentity.Skill) infraskill.RuntimeSkill {
	return infraskill.RuntimeSkill{
		ID:          sk.ID,
		SpaceID:     sk.SpaceID,
		Name:        sk.Name,
		Description: sk.Description,
		ObjectKey:   fmt.Sprintf("skills/%d/%d/%s", sk.SpaceID, sk.ID, filepath.Base(sk.FileName)),
	}
}
