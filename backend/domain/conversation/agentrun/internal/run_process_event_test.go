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

package internal

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"

	agentrun "github.com/coze-dev/coze-studio/backend/crossdomain/agentrun/model"
	"github.com/coze-dev/coze-studio/backend/domain/conversation/agentrun/entity"
)

type fakeRunRecordRepo struct {
	lastID     int64
	lastUpdate *entity.UpdateMeta
}

func (f *fakeRunRecordRepo) Create(_ context.Context, _ *entity.AgentRunMeta) (*entity.RunRecordMeta, error) {
	return nil, nil
}

func (f *fakeRunRecordRepo) GetByID(_ context.Context, _ int64) (*entity.RunRecordMeta, error) {
	return nil, nil
}

func (f *fakeRunRecordRepo) Cancel(_ context.Context, _ *entity.CancelRunMeta) (*entity.RunRecordMeta, error) {
	return nil, nil
}

func (f *fakeRunRecordRepo) Delete(_ context.Context, _ []int64) error {
	return nil
}

func (f *fakeRunRecordRepo) UpdateByID(_ context.Context, id int64, update *entity.UpdateMeta) error {
	f.lastID = id
	f.lastUpdate = update
	return nil
}

func (f *fakeRunRecordRepo) List(_ context.Context, _ *entity.ListRunRecordMeta) ([]*entity.RunRecordMeta, error) {
	return nil, nil
}

func newStreamWriterForTest() *schema.StreamWriter[*entity.AgentRunResponse] {
	sr, sw := schema.Pipe[*entity.AgentRunResponse](10)
	go func() {
		for {
			if _, err := sr.Recv(); err != nil {
				return
			}
		}
	}()
	return sw
}

func newRunProcessForTest(repo *fakeRunRecordRepo) *RunProcess {
	return &RunProcess{
		event:         NewMessageEvent(),
		RunRecordRepo: repo,
	}
}

func TestStepToFailedPersistsAccumulatedUsage(t *testing.T) {
	repo := &fakeRunRecordRepo{}
	rp := newRunProcessForTest(repo)
	sw := newStreamWriterForTest()
	defer sw.Close()

	srRecord := &entity.ChunkRunItem{
		ID:    7685626220033081344,
		Error: &entity.RunError{Code: 500, Msg: "tool call failed"},
	}
	usage := &agentrun.Usage{
		LlmPromptTokens:     11540,
		LlmCompletionTokens: 252,
		LlmTotalTokens:      11792,
	}

	rp.StepToFailed(context.Background(), srRecord, sw, usage)

	assert.Equal(t, srRecord.ID, repo.lastID)
	assert.NotNil(t, repo.lastUpdate)
	assert.Equal(t, entity.RunStatusFailed, repo.lastUpdate.Status)
	assert.NotNil(t, repo.lastUpdate.Usage)
	assert.Equal(t, int64(11540), repo.lastUpdate.Usage.LlmPromptTokens)
	assert.Equal(t, int64(252), repo.lastUpdate.Usage.LlmCompletionTokens)
	assert.Equal(t, int64(11792), repo.lastUpdate.Usage.LlmTotalTokens)
	assert.NotZero(t, repo.lastUpdate.FailedAt)
	assert.Zero(t, repo.lastUpdate.CompletedAt)
	assert.Equal(t, entity.RunStatusFailed, srRecord.Status)
}

func TestStepToFailedWithNilUsageLeavesUsageUnset(t *testing.T) {
	repo := &fakeRunRecordRepo{}
	rp := newRunProcessForTest(repo)
	sw := newStreamWriterForTest()
	defer sw.Close()

	srRecord := &entity.ChunkRunItem{
		ID:    1,
		Error: &entity.RunError{Code: 400, Msg: "bad input"},
	}

	rp.StepToFailed(context.Background(), srRecord, sw, nil)

	assert.NotNil(t, repo.lastUpdate)
	assert.Equal(t, entity.RunStatusFailed, repo.lastUpdate.Status)
	assert.Nil(t, repo.lastUpdate.Usage)
}

func TestStepToCompleteStillPersistsUsage(t *testing.T) {
	repo := &fakeRunRecordRepo{}
	rp := newRunProcessForTest(repo)
	sw := newStreamWriterForTest()
	defer sw.Close()

	srRecord := &entity.ChunkRunItem{ID: 2}
	usage := &agentrun.Usage{
		LlmPromptTokens:     100,
		LlmCompletionTokens: 20,
		LlmTotalTokens:      120,
	}

	rp.StepToComplete(context.Background(), srRecord, sw, usage)

	assert.NotNil(t, repo.lastUpdate)
	assert.Equal(t, entity.RunStatusCompleted, repo.lastUpdate.Status)
	assert.NotNil(t, repo.lastUpdate.Usage)
	assert.Equal(t, int64(120), repo.lastUpdate.Usage.LlmTotalTokens)
	assert.NotZero(t, repo.lastUpdate.CompletedAt)
	assert.Zero(t, repo.lastUpdate.FailedAt)
}
