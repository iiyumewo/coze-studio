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

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"

	singleagent "github.com/coze-dev/coze-studio/backend/crossdomain/agent/model"
)

const (
	toolActivityPhaseStarted  = "started"
	toolActivityPhaseFinished = "finished"
	toolActivityPhaseFailed   = "failed"
)

type toolActivityMeta struct {
	ToolName     string
	RunningText  string
	FinishedText string
	FailedText   string
}

type toolActivityEmitter func(*singleagent.ToolActivityEvent)

type toolActivityEmitterContextKey struct{}

func withToolActivityEmitter(ctx context.Context, emitter toolActivityEmitter) context.Context {
	return context.WithValue(ctx, toolActivityEmitterContextKey{}, emitter)
}

func toolActivityEmitterFromContext(ctx context.Context) toolActivityEmitter {
	emitter, _ := ctx.Value(toolActivityEmitterContextKey{}).(toolActivityEmitter)
	return emitter
}

type activityInvokableTool struct {
	tool.InvokableTool
	activity toolActivityMeta
}

func withToolActivity(base tool.InvokableTool, activity toolActivityMeta) tool.InvokableTool {
	if base == nil {
		return nil
	}
	return &activityInvokableTool{
		InvokableTool: base,
		activity:      activity,
	}
}

func (t *activityInvokableTool) InvokableRun(
	ctx context.Context,
	argumentsInJSON string,
	opts ...tool.Option,
) (string, error) {
	emitter := toolActivityEmitterFromContext(ctx)
	if emitter == nil {
		return t.InvokableTool.InvokableRun(ctx, argumentsInJSON, opts...)
	}

	activityID := compose.GetToolCallID(ctx)
	if activityID == "" {
		activityID = uuid.NewString()
	}
	emitter(&singleagent.ToolActivityEvent{
		ActivityID: activityID,
		ToolName:   t.activity.ToolName,
		Phase:      toolActivityPhaseStarted,
		Text:       t.activity.RunningText,
	})

	result, err := t.InvokableTool.InvokableRun(ctx, argumentsInJSON, opts...)
	phase := toolActivityPhaseFinished
	text := t.activity.FinishedText
	if err != nil {
		phase = toolActivityPhaseFailed
		text = t.activity.FailedText
	}
	emitter(&singleagent.ToolActivityEvent{
		ActivityID: activityID,
		ToolName:   t.activity.ToolName,
		Phase:      phase,
		Text:       text,
	})
	return result, err
}
