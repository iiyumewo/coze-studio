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
	"testing"

	"github.com/stretchr/testify/assert"

	agentrun "github.com/coze-dev/coze-studio/backend/crossdomain/agentrun/model"
	msgEntity "github.com/coze-dev/coze-studio/backend/domain/conversation/message/entity"
)

func TestAgentRuntimeAddUsageExtAccumulatesAcrossModelCalls(t *testing.T) {
	art := &AgentRuntime{}

	art.AddUsageExt(&msgEntity.UsageExt{InputTokens: 100, OutputTokens: 10, TotalCount: 110})
	art.AddUsageExt(&msgEntity.UsageExt{InputTokens: 150, OutputTokens: 20, TotalCount: 170})
	art.AddUsageExt(&msgEntity.UsageExt{InputTokens: 200, OutputTokens: 30, TotalCount: 230})

	usage := art.GetUsage()
	assert.NotNil(t, usage)
	assert.Equal(t, int64(450), usage.LlmPromptTokens)
	assert.Equal(t, int64(60), usage.LlmCompletionTokens)
	assert.Equal(t, int64(510), usage.LlmTotalTokens)
}

func TestAgentRuntimeAddUsageExtIgnoresNil(t *testing.T) {
	art := &AgentRuntime{}

	art.AddUsageExt(nil)
	assert.Nil(t, art.GetUsage())

	art.AddUsageExt(&msgEntity.UsageExt{InputTokens: 10, OutputTokens: 5, TotalCount: 15})
	art.AddUsageExt(nil)

	usage := art.GetUsage()
	assert.NotNil(t, usage)
	assert.Equal(t, int64(15), usage.LlmTotalTokens)
}

func TestAgentRuntimeAddUsageInitializesNilBase(t *testing.T) {
	art := &AgentRuntime{}

	art.AddUsage(&agentrun.Usage{
		LlmPromptTokens:     7,
		LlmCompletionTokens: 3,
		LlmTotalTokens:      10,
	})

	usage := art.GetUsage()
	assert.NotNil(t, usage)
	assert.Equal(t, int64(7), usage.LlmPromptTokens)
	assert.Equal(t, int64(3), usage.LlmCompletionTokens)
	assert.Equal(t, int64(10), usage.LlmTotalTokens)
}

func TestAgentRuntimeSetUsageStillOverwrites(t *testing.T) {
	art := &AgentRuntime{}

	art.AddUsageExt(&msgEntity.UsageExt{InputTokens: 100, OutputTokens: 10, TotalCount: 110})
	art.SetUsage(&agentrun.Usage{
		LlmPromptTokens:     500,
		LlmCompletionTokens: 50,
		LlmTotalTokens:      550,
	})

	usage := art.GetUsage()
	assert.NotNil(t, usage)
	assert.Equal(t, int64(500), usage.LlmPromptTokens)
	assert.Equal(t, int64(550), usage.LlmTotalTokens)
}
