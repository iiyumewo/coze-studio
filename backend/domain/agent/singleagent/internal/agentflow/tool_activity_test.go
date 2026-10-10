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

package agentflow

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/stretchr/testify/require"

	singleagent "github.com/coze-dev/coze-studio/backend/crossdomain/agent/model"
)

type activityTestInput struct {
	Value string `json:"value"`
}

func TestActivityInvokableToolEmitsOneUpdatableActivity(t *testing.T) {
	base, err := utils.InferTool("test_tool", "test", func(_ context.Context, input *activityTestInput) (string, error) {
		return input.Value, nil
	})
	require.NoError(t, err)

	wrapped := withToolActivity(base, toolActivityMeta{
		ToolName:     "test_tool",
		RunningText:  "running",
		FinishedText: "finished",
		FailedText:   "failed",
	})
	events := make([]*singleagent.ToolActivityEvent, 0, 2)
	ctx := withToolActivityEmitter(context.Background(), func(event *singleagent.ToolActivityEvent) {
		events = append(events, event)
	})

	result, err := wrapped.InvokableRun(ctx, `{"value":"ok"}`)

	require.NoError(t, err)
	require.Equal(t, "ok", result)
	require.Len(t, events, 2)
	require.NotEmpty(t, events[0].ActivityID)
	require.Equal(t, events[0].ActivityID, events[1].ActivityID)
	require.Equal(t, toolActivityPhaseStarted, events[0].Phase)
	require.Equal(t, "running", events[0].Text)
	require.Equal(t, toolActivityPhaseFinished, events[1].Phase)
	require.Equal(t, "finished", events[1].Text)
}
