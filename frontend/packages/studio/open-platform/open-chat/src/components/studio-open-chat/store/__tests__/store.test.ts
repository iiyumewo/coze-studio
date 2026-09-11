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
import { type Conversation } from '@coze/api';

import { OpenApiSource } from '@/types/open';

import { createChatStore } from '../store';

const createConversation = (
  overrides: Partial<Conversation> = {},
): Conversation => ({
  id: 'conversation-1',
  last_section_id: 'section-1',
  created_at: 1,
  updated_at: 1,
  meta_data: {},
  ...overrides,
});

describe('chat store conversations', () => {
  test('keeps an existing conversation name when adding the same id', () => {
    const store = createChatStore({
      bot_id: 'bot-1',
      conversation_id: 'conversation-1',
      source: OpenApiSource.WebSdk,
    });

    store
      .getState()
      .updateConversations([createConversation({ name: '已命名会话' })], 'add');
    store
      .getState()
      .updateConversations([createConversation({ updated_at: 2 })], 'add');

    expect(store.getState().conversations).toHaveLength(1);
    expect(store.getState().conversations[0]?.name).toBe('已命名会话');
    expect(store.getState().conversations[0]?.updated_at).toBe(2);
  });
});
