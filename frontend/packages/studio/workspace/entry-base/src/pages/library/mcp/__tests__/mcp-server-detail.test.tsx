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

import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
} from '@testing-library/react';
import {
  ActionKey,
  type ResourceInfo,
} from '@coze-arch/bot-api/plugin_develop';

import { useMcpConfig } from '../../hooks/use-entity-configs/use-mcp-config';

const mocks = vi.hoisted(() => ({
  deleteServer: vi.fn(),
  getMcpServerDetail: vi.fn(),
  toastSuccess: vi.fn(),
}));

vi.mock('@coze-arch/bot-api', () => ({
  mcpApi: {
    deleteServer: mocks.deleteServer,
  },
}));

vi.mock('@coze-arch/i18n', () => ({
  I18n: {
    t: (key: string, values?: { n?: number }) =>
      values?.n === undefined ? key : `${values.n} tools`,
  },
}));

vi.mock('@coze-arch/coze-design/icons', () => {
  const moduleExports: Record<string, unknown> = {};
  moduleExports.IconCozLink = () => <span>link</span>;
  moduleExports.IconCozRefresh = () => <span>refresh</span>;
  return moduleExports;
});

vi.mock('@coze-arch/coze-design', () => {
  const Skeleton = ({ children }: { children?: React.ReactNode }) => (
    <div>{children}</div>
  );
  Skeleton.Title = () => <span>loading title</span>;
  Skeleton.Paragraph = () => <span>loading content</span>;

  const moduleExports: Record<string, unknown> = {};
  const menu: Record<string, unknown> = {};

  moduleExports.Button = ({
    children,
    onClick,
  }: {
    children?: React.ReactNode;
    onClick?: () => void;
  }) => <button onClick={onClick}>{children}</button>;
  menu.Item = ({ children }: { children?: React.ReactNode }) => (
    <div>{children}</div>
  );
  moduleExports.Menu = menu;
  moduleExports.Modal = ({
    children,
    title,
  }: {
    children?: React.ReactNode;
    title?: React.ReactNode;
  }) => (
    <div role="dialog">
      <h2>{title}</h2>
      {children}
    </div>
  );
  moduleExports.Skeleton = Skeleton;
  moduleExports.Tag = ({ children }: { children?: React.ReactNode }) => (
    <span>{children}</span>
  );
  const table: Record<string, unknown> = {};
  table.TableAction = ({
    deleteProps,
  }: {
    deleteProps?: { disabled?: boolean; handler?: () => Promise<void> };
  }) => (
    <button
      type="button"
      disabled={deleteProps?.disabled}
      onClick={() => void deleteProps?.handler?.()}
    >
      delete
    </button>
  );
  moduleExports.Table = table;
  moduleExports.Toast = {
    success: mocks.toastSuccess,
  };

  return moduleExports;
});

vi.mock('@coze-arch/bot-utils', () => ({
  formatDate: (timestamp: number) => String(timestamp),
}));

vi.mock('../create-mcp-server-modal', () => {
  const moduleExports: Record<string, unknown> = {};
  moduleExports.CreateMcpServerModal = () => <div>create MCP server</div>;
  return moduleExports;
});

vi.mock('../mcp-library-item', () => {
  const moduleExports: Record<string, unknown> = {};
  moduleExports.McpLibraryItem = () => <div>MCP library item</div>;
  return moduleExports;
});

vi.mock('../get-mcp-server', () => ({
  getMcpServerDetail: mocks.getMcpServerDetail,
}));

describe('MCP server detail interaction', () => {
  beforeEach(() => {
    mocks.deleteServer.mockReset();
    mocks.deleteServer.mockResolvedValue({ code: 0, msg: 'success' });
    mocks.getMcpServerDetail.mockReset();
    mocks.getMcpServerDetail.mockResolvedValue({
      id: 'mcp-1',
      name: 'Playwright',
      description: 'Browser automation',
      server_url: 'http://127.0.0.1:8931/mcp',
      transport: 'streamable_http',
      authentication: {
        type: 'header',
        header_name: 'Authorization',
      },
      tools: [
        {
          name: 'browser_navigate',
          title: 'Navigate',
          description: 'Open a URL',
        },
      ],
      protocol_version: '2025-06-18',
      server_version: '0.0.79',
      status: 1,
      validation_status: 'success',
      last_checked_at: 100,
      created_at: 200,
      updated_at: 300,
    });
    mocks.toastSuccess.mockReset();
  });

  it('opens and renders the selected MCP server detail', async () => {
    const { result } = renderHook(() =>
      useMcpConfig({ spaceId: 'space-1', reloadList: vi.fn() }),
    );

    act(() => {
      result.current.config.onItemClick({
        res_id: 'mcp-1',
      } as ResourceInfo);
    });

    render(<>{result.current.modals}</>);

    expect(await screen.findByText('Playwright')).toBeTruthy();
    expect(screen.getByText('browser_navigate')).toBeTruthy();
    expect(screen.getByText('Authorization', { exact: false })).toBeTruthy();
    expect(screen.getByText('Open a URL')).toBeTruthy();
    expect(mocks.getMcpServerDetail).toHaveBeenCalledWith('space-1', 'mcp-1');
  });

  it('deletes an enabled MCP server and refreshes the resource list', async () => {
    const reloadList = vi.fn();
    const { result } = renderHook(() =>
      useMcpConfig({ spaceId: 'space-1', reloadList }),
    );

    render(
      <>
        {result.current.config.renderActions({
          res_id: 'mcp-1',
          actions: [{ key: ActionKey.Delete, enable: true }],
        } as ResourceInfo)}
      </>,
    );
    fireEvent.click(screen.getByRole('button', { name: 'delete' }));

    await waitFor(() => {
      expect(mocks.deleteServer).toHaveBeenCalledWith('mcp-1', 'space-1');
    });
    expect(reloadList).toHaveBeenCalledTimes(1);
    expect(mocks.toastSuccess).toHaveBeenCalledWith('Delete_success');
  });
});
