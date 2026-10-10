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

import { describe, expect, it, vi } from 'vitest';
import { type ChatV3Message, type ListMessageData } from '@coze/api';

import { messageConverterToCoze } from './message-convert-to-coze';
import { createToolActivityMessage } from './message-activity';

vi.mock('@coze-common/chat-core', () => ({
  ContentType: {
    Text: 'text',
    Card: 'card',
    Mix: 'mix',
    File: 'file',
  },
}));

vi.mock('@coze-studio/file-kit/logic', () => ({
  FileTypeEnum: { DEFAULT_UNKNOWN: 'default_unknown' },
  getFileInfo: () => ({ fileType: 'default_unknown' }),
}));

const fileContent = JSON.stringify({
  file_list: [
    {
      file_key: 'skill-artifacts/conversation/report.docx',
      file_name: 'report.docx',
      file_type: 'docx',
      file_size: 1024,
      file_url: 'https://files.example.com/report.docx',
    },
  ],
});

const fileMessage = {
  id: 'message-1',
  chat_id: 'chat-1',
  bot_id: 'bot-1',
  role: 'assistant',
  type: 'answer',
  content_type: 'file',
  content: fileContent,
  created_at: 1,
} as unknown as ChatV3Message;

describe('messageConverterToCoze file messages', () => {
  it('converts a streamed file message for FileCard rendering', () => {
    const message = messageConverterToCoze.convertMessage(fileMessage);

    expect(message.content_type).toBe('file');
    expect(message.content).toBe(fileContent);
  });

  it('keeps file messages when converting conversation history', () => {
    const result = messageConverterToCoze.convertMessageListResponse(
      {
        data: [fileMessage],
        has_more: false,
        first_id: 'message-1',
        last_id: 'message-1',
      } as ListMessageData,
      'bot-1',
    );

    expect(result.message_list).toHaveLength(1);
    expect(result.message_list[0].content_type).toBe('file');
    expect(result.message_list[0].content).toBe(fileContent);
  });
});

describe('MessageParser tool activities', () => {
  it('uses one message id while changing pending to completed', () => {
    const baseActivity = {
      activity_id: 'call-1',
      tool_name: 'file_download',
      conversation_id: 'conversation-1',
      chat_id: 'chat-1',
      bot_id: 'bot-1',
      section_id: 'section-1',
      created_at: 1,
    };

    const context = {
      conversationId: 'conversation-1',
      botId: 'bot-1',
      sectionId: 'section-1',
      seqId: 1,
    };
    const started = createToolActivityMessage(
      {
        ...baseActivity,
        phase: 'started',
        text: '正在准备附件',
      },
      context,
    );
    const finished = createToolActivityMessage(
      {
        ...baseActivity,
        phase: 'finished',
        text: '附件已准备',
      },
      { ...context, seqId: 2 },
    );

    expect(started?.data.message.message_id).toBe('call-1');
    expect(finished?.data.message.message_id).toBe('call-1');
    expect(started?.data.message.content).toBe('● 正在准备附件');
    expect(finished?.data.message.content).toBe('✓ 附件已准备');
    expect(started?.data.is_finish).toBe(false);
    expect(finished?.data.is_finish).toBe(true);
  });
});
