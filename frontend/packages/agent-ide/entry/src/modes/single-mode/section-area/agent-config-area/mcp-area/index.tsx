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

import { type FC, useState } from 'react';

import { useBotSkillStore } from '@coze-studio/bot-detail-store/bot-skill';
import { I18n } from '@coze-arch/i18n';
import { IconCozLink } from '@coze-arch/coze-design/icons';
import { AddButton, ToolContentBlock } from '@coze-agent-ide/tool';

import { McpSelectorModal } from './mcp-selector-modal';

interface McpAreaProps {
  title: string;
}

export const McpArea: FC<McpAreaProps> = ({ title }) => {
  const mcpServers = useBotSkillStore(state => state.mcpServers);
  const updateSkillMCPServers = useBotSkillStore(
    state => state.updateSkillMCPServers,
  );
  const [selectorVisible, setSelectorVisible] = useState(false);

  return (
    <>
      <ToolContentBlock
        header={title}
        defaultExpand={mcpServers.length > 0}
        actionButton={
          <AddButton
            tooltips={I18n.t('bot_edit_mcp_select_title')}
            onClick={() => setSelectorVisible(true)}
            enableAutoHidden={true}
            data-testid="bot.editor.tool.mcp.add-button"
          />
        }
      >
        <div className="px-[8px]">
          {mcpServers.length ? (
            <div className="flex flex-col gap-[8px]">
              {mcpServers.map(server => (
                <div
                  key={server.mcp_server_id}
                  data-testid="bot.editor.tool.mcp.item"
                  className="flex min-h-[48px] items-center gap-[12px] rounded-[6px] border border-solid coz-stroke-primary px-[12px] py-[8px]"
                >
                  <div className="flex h-[32px] w-[32px] flex-none items-center justify-center rounded-[6px] coz-mg-hglt">
                    <IconCozLink className="h-[18px] w-[18px] coz-fg-hglt" />
                  </div>
                  <div className="min-w-0">
                    <div className="text-sm font-medium leading-[20px] coz-fg-primary">
                      {I18n.t('library_resource_type_mcp')}
                    </div>
                    <div className="truncate text-xs leading-[18px] coz-fg-secondary">
                      {server.mcp_server_id}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-sm leading-[22px] coz-fg-secondary">
              {I18n.t('bot_edit_mcp_explain')}
            </div>
          )}
        </div>
      </ToolContentBlock>
      {selectorVisible ? (
        <McpSelectorModal
          selectedServerIds={mcpServers.map(server => server.mcp_server_id)}
          onCancel={() => setSelectorVisible(false)}
          onConfirm={serverIds => {
            updateSkillMCPServers(
              serverIds.map(serverId => ({ mcp_server_id: serverId })),
            );
            setSelectorVisible(false);
          }}
        />
      ) : null}
    </>
  );
};
