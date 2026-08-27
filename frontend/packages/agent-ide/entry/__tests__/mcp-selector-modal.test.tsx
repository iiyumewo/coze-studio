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
import { fireEvent, render, screen, waitFor } from '@testing-library/react';

import { McpSelectorModal } from '../src/modes/single-mode/section-area/agent-config-area/mcp-area/mcp-selector-modal';

const mocks = vi.hoisted(() => ({
  libraryResourceList: vi.fn(),
}));

vi.mock('@coze-arch/bot-api', () => ({
  PluginDevelopApi: {
    LibraryResourceList: mocks.libraryResourceList,
  },
}));

vi.mock('@coze-arch/bot-studio-store', () => ({
  useSpaceStore: (selector: (state: { space: { id: string } }) => unknown) =>
    selector({ space: { id: 'space-1' } }),
}));

vi.mock('@coze-arch/coze-design/icons', () => ({
  ['IconCozEmpty']: () => <span />,
  ['IconCozLink']: () => <span />,
}));

vi.mock('@coze-arch/coze-design', () => ({
  ['Button']: ({
    children,
    onClick,
  }: {
    children: React.ReactNode;
    onClick: () => void;
  }) => (
    <button type="button" onClick={onClick}>
      {children}
    </button>
  ),
  ['Checkbox']: ({
    checked,
    children,
    onChange,
  }: {
    checked: boolean;
    children: React.ReactNode;
    onChange: React.ChangeEventHandler<HTMLInputElement>;
  }) => (
    <label>
      <input type="checkbox" checked={checked} onChange={onChange} />
      {children}
    </label>
  ),
  ['EmptyState']: ({ title }: { title: React.ReactNode }) => <div>{title}</div>,
  ['Modal']: ({
    title,
    children,
    onCancel,
    onOk,
    okButtonProps,
  }: {
    title: React.ReactNode;
    children: React.ReactNode;
    onCancel: () => void;
    onOk: () => void;
    okButtonProps?: { disabled?: boolean };
  }) => (
    <div role="dialog">
      <h2>{title}</h2>
      {children}
      <button type="button" onClick={onCancel}>
        Cancel
      </button>
      <button type="button" disabled={okButtonProps?.disabled} onClick={onOk}>
        Confirm
      </button>
    </div>
  ),
  ['Spin']: () => <div>Loading</div>,
}));

vi.mock('@coze-arch/i18n', () => ({
  I18n: {
    t: (key: string, variables?: { n?: number }) =>
      ({
        Cancel: 'Cancel',
        Confirm: 'Confirm',
        Retry: 'Retry',
        bot_edit_mcp_select_empty: 'No MCP services',
        bot_edit_mcp_select_load_failed: 'Failed to load MCP services',
        bot_edit_mcp_select_title: 'Select MCP services',
        bot_edit_page_plugin_list_plugin_has_n_tools: `Tools: ${variables?.n}`,
      })[key] ?? key,
  },
}));

describe('McpSelectorModal', () => {
  beforeEach(() => {
    mocks.libraryResourceList.mockReset();
  });

  it('loads every page and confirms only available MCP servers', async () => {
    mocks.libraryResourceList
      .mockResolvedValueOnce({
        resource_list: [
          {
            res_id: '100',
            name: 'First MCP',
            biz_res_status: 1,
            biz_extend: { tool_count: '5' },
          },
          { res_id: '200', name: 'Disabled MCP', biz_res_status: 2 },
        ],
        cursor: 'next-page',
        has_more: true,
      })
      .mockResolvedValueOnce({
        resource_list: [
          { res_id: '300', name: 'Second MCP', biz_res_status: 1 },
          {
            res_id: '400',
            name: 'Deleted MCP',
            biz_res_status: 1,
            del_flag: true,
          },
        ],
        has_more: false,
      });
    const onConfirm = vi.fn();

    render(
      <McpSelectorModal
        selectedServerIds={['100', '200']}
        onCancel={vi.fn()}
        onConfirm={onConfirm}
      />,
    );

    const firstMcp = await screen.findByLabelText(/First MCP/);
    const secondMcp = screen.getByLabelText(/Second MCP/);
    expect((firstMcp as HTMLInputElement).checked).toBe(true);
    expect((secondMcp as HTMLInputElement).checked).toBe(false);
    expect(screen.queryByText('Disabled MCP')).toBeNull();
    expect(screen.queryByText('Deleted MCP')).toBeNull();
    expect(screen.getByText('Tools: 5')).toBeTruthy();
    expect(mocks.libraryResourceList).toHaveBeenNthCalledWith(1, {
      space_id: 'space-1',
      res_type_filter: [10],
      size: 100,
      cursor: undefined,
    });
    expect(mocks.libraryResourceList).toHaveBeenNthCalledWith(2, {
      space_id: 'space-1',
      res_type_filter: [10],
      size: 100,
      cursor: 'next-page',
    });

    fireEvent.click(secondMcp);
    fireEvent.click(screen.getByText('Confirm'));

    expect(onConfirm).toHaveBeenCalledWith([
      {
        mcp_server_id: '100',
        name: 'First MCP',
        status: 1,
        tool_count: 5,
      },
      {
        mcp_server_id: '300',
        name: 'Second MCP',
        status: 1,
        tool_count: undefined,
      },
    ]);
  });

  it('keeps the draft unchanged when cancelled', async () => {
    mocks.libraryResourceList.mockResolvedValue({
      resource_list: [{ res_id: '100', name: 'First MCP', biz_res_status: 1 }],
      has_more: false,
    });
    const onCancel = vi.fn();
    const onConfirm = vi.fn();

    render(
      <McpSelectorModal
        selectedServerIds={['100']}
        onCancel={onCancel}
        onConfirm={onConfirm}
      />,
    );

    await screen.findByLabelText(/First MCP/);
    fireEvent.click(screen.getByText('Cancel'));

    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('shows a retry action after the resource list request fails', async () => {
    mocks.libraryResourceList
      .mockRejectedValueOnce(new Error('network error'))
      .mockResolvedValueOnce({ resource_list: [], has_more: false });

    render(
      <McpSelectorModal
        selectedServerIds={[]}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );

    expect(await screen.findByText('Failed to load MCP services')).toBeTruthy();
    fireEvent.click(screen.getByText('Retry'));

    await waitFor(() =>
      expect(mocks.libraryResourceList).toHaveBeenCalledTimes(2),
    );
    expect(await screen.findByText('No MCP services')).toBeTruthy();
  });
});
