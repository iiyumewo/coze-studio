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

import { describe, expect, test } from 'vitest';
import { type Message } from '@coze-common/chat-area';

import { getConversationName } from '../conversation-name';

const createMessage = (
  contentType: Message['content_type'],
  content: string,
): Message =>
  ({
    content_type: contentType,
    content,
  }) as Message;

describe('getConversationName', () => {
  test('uses text content', () => {
    expect(getConversationName(createMessage('text', '  测试连接  '))).toBe(
      '测试连接',
    );
  });

  test('extracts text from a mixed message', () => {
    expect(
      getConversationName(
        createMessage(
          'mix',
          JSON.stringify({
            item_list: [
              { type: 'image', file_id: 'image-1' },
              { type: 'text', text: '请审核这张图片' },
            ],
          }),
        ),
      ),
    ).toBe('请审核这张图片');
  });

  test('uses the file message name when no text is available', () => {
    expect(
      getConversationName(
        createMessage(
          'mix',
          JSON.stringify({
            item_list: [{ type: 'image', file_id: 'image-1' }],
          }),
        ),
      ),
    ).toBe('文件消息');
  });

  test('uses the file message name for invalid mixed content', () => {
    expect(getConversationName(createMessage('mix', 'invalid-json'))).toBe(
      '文件消息',
    );
  });
});
