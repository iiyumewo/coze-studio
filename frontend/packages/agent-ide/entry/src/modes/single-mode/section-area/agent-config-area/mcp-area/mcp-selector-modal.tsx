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

import { useRequest } from 'ahooks';
import { I18n } from '@coze-arch/i18n';
import { IconCozEmpty, IconCozLink } from '@coze-arch/coze-design/icons';
import {
  Button,
  Checkbox,
  EmptyState,
  Modal,
  Spin,
} from '@coze-arch/coze-design';
import { useSpaceStore } from '@coze-arch/bot-studio-store';
import { ResType, type ResourceInfo } from '@coze-arch/bot-api/plugin_develop';
import { PluginDevelopApi } from '@coze-arch/bot-api';

interface McpSelectorModalProps {
  selectedServerIds: string[];
  onCancel: () => void;
  onConfirm: (serverIds: string[]) => void;
}

type AvailableMcpServer = ResourceInfo & { res_id: string };

const MCP_PAGE_SIZE = 100;
const MCP_SERVER_STATUS_ENABLED = 1;

const isAvailableMcpServer = (
  resource: ResourceInfo,
): resource is AvailableMcpServer =>
  typeof resource.res_id === 'string' &&
  resource.res_id.length > 0 &&
  resource.biz_res_status === MCP_SERVER_STATUS_ENABLED &&
  resource.del_flag !== true;

const fetchMcpServerPages = async (
  spaceId: string,
  cursor?: string,
  visitedCursors = new Set<string>(),
): Promise<ResourceInfo[]> => {
  const response = await PluginDevelopApi.LibraryResourceList({
    space_id: spaceId,
    res_type_filter: [ResType.MCP],
    size: MCP_PAGE_SIZE,
    cursor,
  });
  const resources = response.resource_list ?? [];
  const nextCursor = response.cursor;

  if (
    !response.has_more ||
    !nextCursor ||
    nextCursor === cursor ||
    visitedCursors.has(nextCursor)
  ) {
    return resources;
  }

  visitedCursors.add(nextCursor);
  return [
    ...resources,
    ...(await fetchMcpServerPages(spaceId, nextCursor, visitedCursors)),
  ];
};

const fetchAvailableMcpServers = async (
  spaceId: string,
): Promise<AvailableMcpServer[]> => {
  const resources = await fetchMcpServerPages(spaceId);
  return resources.filter(isAvailableMcpServer);
};

const parseToolCount = (value?: string) => {
  if (!value?.trim()) {
    return undefined;
  }

  const toolCount = Number(value);
  return Number.isInteger(toolCount) && toolCount >= 0 ? toolCount : undefined;
};

export const McpSelectorModal: FC<McpSelectorModalProps> = ({
  selectedServerIds,
  onCancel,
  onConfirm,
}) => {
  const spaceId = useSpaceStore(state => state.space.id) ?? '';
  const [pendingServerIds, setPendingServerIds] = useState(selectedServerIds);
  const {
    data: availableServers = [],
    loading,
    error,
    refresh,
  } = useRequest(() => fetchAvailableMcpServers(spaceId), {
    ready: Boolean(spaceId),
    onError: () => undefined,
    onSuccess: servers => {
      const availableServerIds = new Set(servers.map(server => server.res_id));
      setPendingServerIds(
        selectedServerIds.filter(serverId => availableServerIds.has(serverId)),
      );
    },
  });

  const handleServerChange = (serverId: string, checked: boolean) => {
    setPendingServerIds(currentIds =>
      checked
        ? [...new Set([...currentIds, serverId])]
        : currentIds.filter(currentId => currentId !== serverId),
    );
  };

  return (
    <Modal
      title={I18n.t('bot_edit_mcp_select_title')}
      visible={true}
      width={520}
      centered
      maskClosable={false}
      cancelText={I18n.t('Cancel')}
      okText={I18n.t('Confirm')}
      okButtonProps={{ disabled: loading || Boolean(error) || !spaceId }}
      onCancel={onCancel}
      onOk={() => onConfirm(pendingServerIds)}
    >
      {loading ? (
        <div
          data-testid="bot.editor.tool.mcp.selector.loading"
          className="flex h-[240px] items-center justify-center"
        >
          <Spin />
        </div>
      ) : error || !spaceId ? (
        <div className="flex h-[240px] flex-col items-center justify-center gap-[12px]">
          <div role="alert" className="text-sm coz-fg-secondary">
            {I18n.t('bot_edit_mcp_select_load_failed')}
          </div>
          {spaceId ? (
            <Button size="small" color="primary" onClick={refresh}>
              {I18n.t('Retry')}
            </Button>
          ) : null}
        </div>
      ) : availableServers.length ? (
        <div className="flex max-h-[360px] flex-col gap-[8px] overflow-y-auto pr-[4px]">
          {availableServers.map(server => {
            const toolCount = parseToolCount(server.biz_extend?.tool_count);
            return (
              <Checkbox
                key={server.res_id}
                checked={pendingServerIds.includes(server.res_id)}
                onChange={event =>
                  handleServerChange(
                    server.res_id,
                    Boolean(event.target.checked),
                  )
                }
                className="w-full rounded-[6px] border border-solid coz-stroke-primary px-[12px] py-[10px]"
              >
                <div className="flex min-w-0 items-center gap-[12px]">
                  <div className="flex h-[36px] w-[36px] flex-none items-center justify-center rounded-[6px] coz-mg-hglt">
                    <IconCozLink className="h-[18px] w-[18px] coz-fg-hglt" />
                  </div>
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium leading-[20px] coz-fg-primary">
                      {server.name || server.res_id}
                    </div>
                    {toolCount === undefined ? null : (
                      <div className="text-xs leading-[18px] coz-fg-secondary">
                        {I18n.t(
                          'bot_edit_page_plugin_list_plugin_has_n_tools',
                          { n: toolCount },
                        )}
                      </div>
                    )}
                  </div>
                </div>
              </Checkbox>
            );
          })}
        </div>
      ) : (
        <EmptyState
          className="my-[72px] mx-auto"
          icon={<IconCozEmpty />}
          title={I18n.t('bot_edit_mcp_select_empty')}
        />
      )}
    </Modal>
  );
};
