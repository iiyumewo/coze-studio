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

import { useState } from 'react';

import { I18n } from '@coze-arch/i18n';
import { IconCozLink } from '@coze-arch/coze-design/icons';
import { Menu, Table, Toast } from '@coze-arch/coze-design';
import {
  ActionKey,
  ResType,
  type ResourceInfo,
} from '@coze-arch/bot-api/plugin_develop';
import { mcpApi } from '@coze-arch/bot-api';

import { McpServerDetailModal } from '../../mcp/mcp-server-detail-modal';
import { McpLibraryItem } from '../../mcp/mcp-library-item';
import { CreateMcpServerModal } from '../../mcp/create-mcp-server-modal';
import { type UseEntityConfigHook } from './types';

const { TableAction } = Table;

interface SelectedMcpServer {
  spaceId: string;
  mcpServerId: string;
}

export const useMcpConfig: UseEntityConfigHook = ({
  spaceId,
  reloadList,
  getCommonActions,
}) => {
  const [visible, setVisible] = useState(false);
  const [selectedMcpServer, setSelectedMcpServer] =
    useState<SelectedMcpServer>();

  const detailVisible = selectedMcpServer?.spaceId === spaceId;

  return {
    modals: (
      <>
        {visible ? (
          <CreateMcpServerModal
            spaceId={spaceId}
            onCancel={() => setVisible(false)}
            onSuccess={() => {
              setVisible(false);
              reloadList();
            }}
          />
        ) : null}
        {detailVisible ? (
          <McpServerDetailModal
            key={`${selectedMcpServer.spaceId}-${selectedMcpServer.mcpServerId}`}
            spaceId={selectedMcpServer.spaceId}
            mcpServerId={selectedMcpServer.mcpServerId}
            onCancel={() => setSelectedMcpServer(undefined)}
          />
        ) : null}
      </>
    ),
    config: {
      typeFilter: {
        label: I18n.t('library_resource_type_mcp'),
        value: ResType.MCP,
      },
      renderCreateMenu: () => (
        <Menu.Item
          data-testid="workspace.library.header.create.mcp"
          icon={<IconCozLink />}
          onClick={() => setVisible(true)}
        >
          {I18n.t('library_resource_type_mcp')}
        </Menu.Item>
      ),
      target: [ResType.MCP],
      onItemClick: item => {
        if (!item.res_id) {
          return;
        }
        setSelectedMcpServer({ spaceId, mcpServerId: item.res_id });
      },
      renderItem: item => <McpLibraryItem resourceInfo={item} />,
      renderActions: (item: ResourceInfo) => {
        const deleteDisabled =
          !item.res_id ||
          !item.actions?.find(action => action.key === ActionKey.Delete)
            ?.enable;

        return (
          <TableAction
            deleteProps={{
              disabled: deleteDisabled,
              deleteDesc: I18n.t('library_mcp_delete_desc'),
              handler: async () => {
                if (!item.res_id) {
                  return;
                }

                await mcpApi.deleteServer(item.res_id, spaceId);
                setSelectedMcpServer(current =>
                  current?.mcpServerId === item.res_id ? undefined : current,
                );
                reloadList();
                Toast.success(I18n.t('Delete_success'));
              },
            }}
            actionList={getCommonActions?.(item)}
          />
        );
      },
    },
  };
};
