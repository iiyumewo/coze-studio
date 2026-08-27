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
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullStreamToolCallChecker(t *testing.T) {
	toolCallMessage := schema.AssistantMessage("", []schema.ToolCall{
		{
			ID: "call_1",
			Function: schema.FunctionCall{
				Name:      "browser_navigate",
				Arguments: `{"url":"https://www.toutiao.com"}`,
			},
		},
	})

	tests := []struct {
		name       string
		messages   []*schema.Message
		wantCalled bool
	}{
		{
			name:       "tool call in first chunk",
			messages:   []*schema.Message{toolCallMessage},
			wantCalled: true,
		},
		{
			name: "text before tool call",
			messages: []*schema.Message{
				schema.AssistantMessage("I will open the page first.", nil),
				toolCallMessage,
			},
			wantCalled: true,
		},
		{
			name: "text without tool call",
			messages: []*schema.Message{
				schema.AssistantMessage("This answer does not need a tool.", nil),
			},
			wantCalled: false,
		},
		{
			name: "nil chunk before tool call",
			messages: []*schema.Message{
				nil,
				toolCallMessage,
			},
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := schema.StreamReaderFromArray(tt.messages)

			called, err := fullStreamToolCallChecker(context.Background(), stream)

			require.NoError(t, err)
			assert.Equal(t, tt.wantCalled, called)
		})
	}
}

func TestFullStreamToolCallCheckerReturnsStreamError(t *testing.T) {
	wantErr := errors.New("stream failed")
	stream, writer := schema.Pipe[*schema.Message](1)
	writer.Send(nil, wantErr)
	writer.Close()

	called, err := fullStreamToolCallChecker(context.Background(), stream)

	assert.False(t, called)
	assert.ErrorIs(t, err, wantErr)
}

func TestFullStreamToolCallCheckerRejectsNilStream(t *testing.T) {
	called, err := fullStreamToolCallChecker(context.Background(), nil)

	assert.False(t, called)
	assert.EqualError(t, err, "model output stream is nil")
}
