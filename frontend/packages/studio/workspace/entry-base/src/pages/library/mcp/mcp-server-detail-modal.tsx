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

import { type FC, type ReactNode } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@coze-arch/i18n';
import { IconCozLink, IconCozRefresh } from '@coze-arch/coze-design/icons';
import { Button, Modal, Skeleton, Tag } from '@coze-arch/coze-design';
import { formatDate } from '@coze-arch/bot-utils';
import { type McpServerDetail } from '@coze-arch/bot-api';

import { getMcpServerDetail } from './get-mcp-server';

interface McpServerDetailModalProps {
  spaceId: string;
  mcpServerId: string;
  onCancel: () => void;
}

interface DetailFieldProps {
  label: ReactNode;
  value: ReactNode;
  fullWidth?: boolean;
}

const MCP_SERVER_STATUS_ENABLED = 1;
const MCP_SERVER_STATUS_DISABLED = 2;

const DetailField: FC<DetailFieldProps> = ({ label, value, fullWidth }) => (
  <div className={fullWidth ? 'col-span-full min-w-0' : 'min-w-0'}>
    <dt className="mb-[4px] text-[12px] coz-fg-secondary">{label}</dt>
    <dd className="m-0 min-h-[20px] break-words text-[14px] coz-fg-primary">
      {value}
    </dd>
  </div>
);

const DetailSkeleton = () => (
  <div
    data-testid="workspace.library.mcp.detail.loading"
    className="flex min-h-[420px] flex-col gap-[24px] py-[8px]"
  >
    <Skeleton placeholder={<Skeleton.Title />} className="w-1/3" />
    <Skeleton placeholder={<Skeleton.Paragraph rows={4} />} />
    <Skeleton placeholder={<Skeleton.Title />} className="w-1/4" />
    <Skeleton placeholder={<Skeleton.Paragraph rows={6} />} />
  </div>
);

const formatTimestamp = (timestamp: number) =>
  timestamp > 0 ? formatDate(timestamp, 'YYYY-MM-DD HH:mm') : '-';

const getStatusLabel = (status: number) => {
  if (status === MCP_SERVER_STATUS_ENABLED) {
    return I18n.t('library_mcp_status_enabled');
  }
  if (status === MCP_SERVER_STATUS_DISABLED) {
    return I18n.t('library_mcp_status_disabled');
  }
  return '-';
};

const getAuthenticationLabel = (detail: McpServerDetail) => {
  if (detail.authentication.type === 'header') {
    return detail.authentication.header_name
      ? `${I18n.t('library_mcp_auth_header')} (${detail.authentication.header_name})`
      : I18n.t('library_mcp_auth_header');
  }
  return I18n.t('library_mcp_auth_none');
};

const DetailContent: FC<{ detail: McpServerDetail }> = ({ detail }) => {
  const validationSucceeded = detail.validation_status === 'success';

  return (
    <div
      data-testid="workspace.library.mcp.detail.content"
      className="max-h-[70vh] min-h-[420px] overflow-y-auto pr-[4px]"
    >
      <section className="flex items-start gap-[12px] pb-[20px]">
        <div className="flex h-[40px] w-[40px] shrink-0 items-center justify-center rounded-[8px] coz-mg-hglt">
          <IconCozLink className="h-[20px] w-[20px] coz-fg-hglt" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-[8px]">
            <h3 className="m-0 break-words text-[18px] font-[600] coz-fg-primary">
              {detail.name}
            </h3>
            <Tag
              color={
                detail.status === MCP_SERVER_STATUS_ENABLED ? 'green' : 'grey'
              }
              size="mini"
            >
              {getStatusLabel(detail.status)}
            </Tag>
          </div>
          {detail.description ? (
            <p className="mb-0 mt-[6px] whitespace-pre-wrap break-words text-[13px] coz-fg-secondary">
              {detail.description}
            </p>
          ) : null}
        </div>
      </section>

      <section className="border-0 border-t border-solid coz-stroke-primary py-[20px]">
        <h4 className="mb-[16px] mt-0 text-[14px] font-[600] coz-fg-primary">
          {I18n.t('library_mcp_connection_info')}
        </h4>
        <dl className="m-0 grid grid-cols-1 gap-x-[24px] gap-y-[16px] sm:grid-cols-2">
          <DetailField
            fullWidth
            label={I18n.t('library_mcp_server_url')}
            value={
              <span className="block break-all font-mono text-[13px]">
                {detail.server_url}
              </span>
            }
          />
          <DetailField
            label={I18n.t('library_mcp_transport')}
            value={
              detail.transport === 'streamable_http'
                ? 'Streamable HTTP'
                : detail.transport
            }
          />
          <DetailField
            label={I18n.t('library_mcp_auth_type')}
            value={getAuthenticationLabel(detail)}
          />
          <DetailField
            label={I18n.t('library_mcp_protocol_version')}
            value={detail.protocol_version || '-'}
          />
          <DetailField
            label={I18n.t('library_mcp_server_version')}
            value={detail.server_version || '-'}
          />
          <DetailField
            label={I18n.t('library_mcp_validation_status')}
            value={
              <Tag color={validationSucceeded ? 'green' : 'red'} size="mini">
                {validationSucceeded
                  ? I18n.t('library_mcp_validation_success')
                  : I18n.t('library_mcp_validation_failed')}
              </Tag>
            }
          />
          <DetailField
            label={I18n.t('library_mcp_last_checked_at')}
            value={formatTimestamp(detail.last_checked_at)}
          />
        </dl>
        {detail.last_error ? (
          <div className="mt-[16px] rounded-[6px] coz-bg-secondary px-[12px] py-[10px] text-[12px] coz-fg-hglt-red">
            {detail.last_error}
          </div>
        ) : null}
      </section>

      <section className="border-0 border-t border-solid coz-stroke-primary py-[20px]">
        <div className="mb-[8px] flex items-center justify-between gap-[12px]">
          <h4 className="m-0 text-[14px] font-[600] coz-fg-primary">
            {I18n.t('library_mcp_tools')}
          </h4>
          <span className="text-[12px] coz-fg-secondary">
            {I18n.t('bot_edit_page_plugin_list_plugin_has_n_tools', {
              n: detail.tools.length,
            })}
          </span>
        </div>
        {detail.tools.length ? (
          <div data-testid="workspace.library.mcp.detail.tools">
            {detail.tools.map(tool => (
              <div
                key={tool.name}
                className="border-0 border-b border-solid coz-stroke-primary py-[12px] last:border-b-0"
              >
                <div className="flex flex-wrap items-baseline gap-x-[8px] gap-y-[2px]">
                  <span className="break-all font-mono text-[13px] font-[600] coz-fg-primary">
                    {tool.name}
                  </span>
                  {tool.title ? (
                    <span className="text-[12px] coz-fg-secondary">
                      {tool.title}
                    </span>
                  ) : null}
                </div>
                {tool.description ? (
                  <p className="mb-0 mt-[4px] whitespace-pre-wrap break-words text-[12px] coz-fg-secondary">
                    {tool.description}
                  </p>
                ) : null}
              </div>
            ))}
          </div>
        ) : (
          <div className="py-[24px] text-center text-[13px] coz-fg-secondary">
            {I18n.t('library_mcp_no_tools')}
          </div>
        )}
      </section>

      <section className="grid grid-cols-1 gap-x-[24px] gap-y-[16px] border-0 border-t border-solid coz-stroke-primary pt-[20px] sm:grid-cols-2">
        <DetailField
          label={I18n.t('library_mcp_created_at')}
          value={formatTimestamp(detail.created_at)}
        />
        <DetailField
          label={I18n.t('library_mcp_updated_at')}
          value={formatTimestamp(detail.updated_at)}
        />
      </section>
    </div>
  );
};

export const McpServerDetailModal: FC<McpServerDetailModalProps> = ({
  spaceId,
  mcpServerId,
  onCancel,
}) => {
  const { data, loading, error, refresh } = useRequest(
    () => getMcpServerDetail(spaceId, mcpServerId),
    { refreshDeps: [spaceId, mcpServerId] },
  );

  return (
    <Modal
      title={I18n.t('library_mcp_detail_title')}
      visible={true}
      width={680}
      centered
      maskClosable={false}
      footer={null}
      onCancel={onCancel}
    >
      {loading ? <DetailSkeleton /> : null}
      {!loading && error ? (
        <div
          role="alert"
          data-testid="workspace.library.mcp.detail.error"
          className="flex min-h-[420px] flex-col items-center justify-center gap-[16px] text-center"
        >
          <span className="text-[14px] coz-fg-secondary">
            {I18n.t('library_mcp_detail_load_failed')}
          </span>
          <Button icon={<IconCozRefresh />} color="secondary" onClick={refresh}>
            {I18n.t('retry')}
          </Button>
        </div>
      ) : null}
      {!loading && !error && data ? <DetailContent detail={data} /> : null}
    </Modal>
  );
};
