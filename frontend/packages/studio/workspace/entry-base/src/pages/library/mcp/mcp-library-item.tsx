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

import { I18n } from '@coze-arch/i18n';
import { IconCozLink } from '@coze-arch/coze-design/icons';
import { Tag } from '@coze-arch/coze-design';
import { type ResourceInfo } from '@coze-arch/bot-api/plugin_develop';

import { BaseLibraryItem } from '../components/base-library-item';

interface McpLibraryItemProps {
  resourceInfo: ResourceInfo;
}

const parseToolCount = (value?: string) => {
  if (!value?.trim()) {
    return undefined;
  }

  const toolCount = Number(value);
  return Number.isInteger(toolCount) && toolCount >= 0 ? toolCount : undefined;
};

export const McpLibraryItem: React.FC<McpLibraryItemProps> = ({
  resourceInfo,
}) => {
  const toolCount = parseToolCount(resourceInfo.biz_extend?.tool_count);

  return (
    <BaseLibraryItem
      resourceInfo={resourceInfo}
      customAvatar={
        <div
          data-testid="workspace.library.mcp.item.avatar"
          className="mr-[12px] flex h-[40px] w-[40px] flex-shrink-0 items-center justify-center rounded-[12px] coz-mg-hglt"
        >
          <IconCozLink className="h-[20px] w-[20px] coz-fg-hglt" />
        </div>
      }
      tag={
        toolCount === undefined ? null : (
          <Tag
            data-testid="workspace.library.mcp.item.tool-count"
            color="brand"
            size="mini"
            className="flex-shrink-0 flex-grow-0"
          >
            {I18n.t('bot_edit_page_plugin_list_plugin_has_n_tools', {
              n: toolCount,
            })}
          </Tag>
        )
      }
    />
  );
};
