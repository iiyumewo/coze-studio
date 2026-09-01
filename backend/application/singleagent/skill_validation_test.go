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

package singleagent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-studio/backend/api/model/app/bot_common"
	skillentity "github.com/coze-dev/coze-studio/backend/domain/skill/entity"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

type fakeSkillReader struct {
	getByID func(id int64) (*skillentity.Skill, error)
	calls   int
}

func (f *fakeSkillReader) GetByID(_ context.Context, id int64) (*skillentity.Skill, error) {
	f.calls++
	return f.getByID(id)
}

func TestValidateSkills(t *testing.T) {
	const (
		spaceID = int64(100)
		skillID = int64(200)
	)

	t.Run("accept skills from the agent space", func(t *testing.T) {
		reader := &fakeSkillReader{getByID: func(id int64) (*skillentity.Skill, error) {
			return &skillentity.Skill{ID: id, SpaceID: spaceID}, nil
		}}
		service := &SingleAgentApplicationService{appContext: &ServiceComponents{SkillReader: reader}}

		err := service.validateSkills(context.Background(), spaceID, []*bot_common.SkillInfo{{SkillID: skillID}})

		require.NoError(t, err)
		require.Equal(t, 1, reader.calls)
	})

	t.Run("allow an empty list to clear the configuration", func(t *testing.T) {
		reader := &fakeSkillReader{getByID: func(int64) (*skillentity.Skill, error) {
			return nil, errors.New("unexpected query")
		}}
		service := &SingleAgentApplicationService{appContext: &ServiceComponents{SkillReader: reader}}

		err := service.validateSkills(context.Background(), spaceID, []*bot_common.SkillInfo{})

		require.NoError(t, err)
		require.Zero(t, reader.calls)
	})

	t.Run("reject invalid and duplicate IDs before querying", func(t *testing.T) {
		cases := []struct {
			name string
			refs []*bot_common.SkillInfo
		}{
			{name: "missing reference", refs: []*bot_common.SkillInfo{nil}},
			{name: "zero ID", refs: []*bot_common.SkillInfo{{SkillID: 0}}},
			{name: "duplicate ID", refs: []*bot_common.SkillInfo{{SkillID: skillID}, {SkillID: skillID}}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				reader := &fakeSkillReader{getByID: func(int64) (*skillentity.Skill, error) {
					return nil, errors.New("unexpected query")
				}}
				service := &SingleAgentApplicationService{appContext: &ServiceComponents{SkillReader: reader}}

				err := service.validateSkills(context.Background(), spaceID, tc.refs)

				requireStatusErrorCode(t, err, errno.ErrSkillInvalidParamCode)
				require.Zero(t, reader.calls)
			})
		}
	})

	t.Run("reject a skill from another space", func(t *testing.T) {
		reader := &fakeSkillReader{getByID: func(id int64) (*skillentity.Skill, error) {
			return &skillentity.Skill{ID: id, SpaceID: spaceID + 1}, nil
		}}
		service := &SingleAgentApplicationService{appContext: &ServiceComponents{SkillReader: reader}}

		err := service.validateSkills(context.Background(), spaceID, []*bot_common.SkillInfo{{SkillID: skillID}})

		requireStatusErrorCode(t, err, errno.ErrSkillPermissionCode)
	})

	t.Run("return a skill get error when the repository fails", func(t *testing.T) {
		reader := &fakeSkillReader{getByID: func(int64) (*skillentity.Skill, error) {
			return nil, errors.New("database unavailable")
		}}
		service := &SingleAgentApplicationService{appContext: &ServiceComponents{SkillReader: reader}}

		err := service.validateSkills(context.Background(), spaceID, []*bot_common.SkillInfo{{SkillID: skillID}})

		requireStatusErrorCode(t, err, errno.ErrSkillGetCode)
	})
}
