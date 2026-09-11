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

import { type FC } from 'react';

import { nanoid } from 'nanoid';
import {
  AuthType,
  Language,
  Layout,
  OpenApiSource,
} from '@coze-studio/open-chat/types';
import { WebSdkChat } from '@coze-studio/open-chat';

const uid = nanoid();
const demoUserStorageKey = 'coze-chat-sdk-demo-user-id';
const demoUserId =
  localStorage.getItem(demoUserStorageKey) || `demo-user-${nanoid()}`;
localStorage.setItem(demoUserStorageKey, demoUserId);
const demoUserInfo = {
  id: demoUserId,
  nickname: '演示用户',
  url: 'https://api.dicebear.com/9.x/initials/svg?seed=coze-demo-user',
};

const botConfig = {
  user: demoUserId,
  conversation_id: uid,
  bot_id: process.env.CHAT_APP_INDEX_COZE_BOT_ID || '',
  source: OpenApiSource.WebSdk,
  ui: {
    base: {
      lang: Language.ZH_CN,
      layout: Layout.PC,
    },
    conversations: {
      isNeed: true,
    },
  },
  auth: {
    type: AuthType.TOKEN,
    token: process.env.CHAT_APP_COZE_TOKEN || '',
    onRefreshToken: () => process.env.CHAT_APP_COZE_TOKEN || '',
  },
};

const TestChatDemo: FC = () => (
  <>
    <div
      style={{
        position: 'absolute',
        top: 18,
        left: 50,
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        height: 32,
        color: '#4e5969',
        fontSize: 14,
      }}
    >
      <img
        src={demoUserInfo.url}
        alt="演示用户头像"
        width={28}
        height={28}
        style={{ borderRadius: '50%' }}
      />
      <span>当前模拟用户：</span>
      <strong>{demoUserInfo.nickname}</strong>
      <span>用户 ID：{demoUserInfo.id}</span>
    </div>
    <WebSdkChat
      title="客服小助手"
      chatConfig={botConfig}
      userInfo={demoUserInfo}
      className="absolute"
      useInIframe={false}
      style={{
        position: 'absolute',
        left: 50,
        top: 66,
        width: 900,
        height: 700,
      }}
    />
  </>
);

export default TestChatDemo;
