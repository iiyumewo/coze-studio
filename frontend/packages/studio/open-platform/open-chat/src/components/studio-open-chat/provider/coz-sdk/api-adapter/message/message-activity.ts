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

import { ContentType } from '@coze-common/chat-core';

const toolActivityMessageIndex = -1;

export interface ToolActivityEvent {
  activity_id: string;
  tool_name: string;
  phase: 'started' | 'finished' | 'failed';
  text: string;
  conversation_id: string;
  chat_id: string;
  bot_id: string;
  section_id: string;
  created_at: number;
}

interface ActivityMessageContext {
  conversationId: string;
  botId: string;
  sectionId: string;
  seqId: number;
}

export const createToolActivityMessage = (
  activity: ToolActivityEvent,
  context: ActivityMessageContext,
) => {
  if (!activity.activity_id || !activity.chat_id || !activity.text) {
    return;
  }
  const prefix =
    activity.phase === 'started'
      ? '● '
      : activity.phase === 'failed'
        ? '✕ '
        : '✓ ';
  return {
    event: 'message',
    data: {
      conversation_id: activity.conversation_id || context.conversationId,
      index: toolActivityMessageIndex,
      is_finish: activity.phase !== 'started',
      seq_id: context.seqId,
      message: {
        content: `${prefix}${activity.text}`,
        content_time: (activity.created_at || 0) * 1000,
        content_type: ContentType.Text,
        extra_info: {
          local_message_id: '',
          tool_activity_id: activity.activity_id,
          tool_activity_phase: activity.phase,
          tool_activity_name: activity.tool_name,
        },
        message_id: activity.activity_id,
        reply_id: activity.chat_id,
        role: 'assistant',
        section_id: activity.section_id || context.sectionId,
        sender_id: activity.bot_id || context.botId,
        source: 0,
        status: '',
        type: 'answer',
      },
    },
  };
};
