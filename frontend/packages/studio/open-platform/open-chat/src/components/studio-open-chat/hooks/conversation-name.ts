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

import { type Message } from '@coze-common/chat-area';

import { catchParse } from '@/util';

const maxConversationNameLength = 100;
const fileMessageName = '文件消息';

interface MixContentItem {
  type?: string;
  text?: string;
}

interface MixContent {
  item_list?: MixContentItem[];
}

const getMixText = (content: string) => {
  const parsed = catchParse<MixContentItem[] | MixContent>(content);
  const items = Array.isArray(parsed) ? parsed : parsed?.item_list;
  return items
    ?.filter(item => item?.type === 'text')
    .map(item => item.text?.trim())
    .filter((text): text is string => Boolean(text))
    .join(' ')
    .slice(0, maxConversationNameLength);
};

export const getConversationName = (message?: Message) => {
  if (!message) {
    return '';
  }

  if (message.content_type === 'text') {
    return (
      message.content.trim().slice(0, maxConversationNameLength) ||
      fileMessageName
    );
  }

  if (message.content_type === 'mix') {
    return getMixText(message.content) || fileMessageName;
  }

  return fileMessageName;
};
