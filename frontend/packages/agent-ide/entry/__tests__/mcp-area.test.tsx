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
import { fireEvent, render, screen, within } from '@testing-library/react';

import { McpArea } from '../src/modes/single-mode/section-area/agent-config-area/mcp-area';

const mocks = vi.hoisted(() => ({
  mcpServers: [] as Array<{ mcp_server_id: string }>,
  updateSkillMCPServers: vi.fn(),
}));

vi.mock('@coze-studio/bot-detail-store/bot-skill', () => ({
  useBotSkillStore: (
    selector: (state: {
      mcpServers: Array<{ mcp_server_id: string }>;
      updateSkillMCPServers: typeof mocks.updateSkillMCPServers;
    }) => unknown,
  ) =>
    selector({
      mcpServers: mocks.mcpServers,
      updateSkillMCPServers: mocks.updateSkillMCPServers,
    }),
}));

vi.mock('@coze-agent-ide/tool', () => ({
  ['AddButton']: ({
    onClick,
    ...props
  }: {
    onClick: () => void;
    [key: `data-${string}`]: unknown;
  }) => (
    <button
      type="button"
      data-testid={props['data-testid'] as string}
      onClick={onClick}
    >
      Add MCP
    </button>
  ),
  ['ToolContentBlock']: ({
    header,
    children,
    actionButton,
    defaultExpand,
  }: {
    header: React.ReactNode;
    children: React.ReactNode;
    actionButton?: React.ReactNode;
    defaultExpand?: boolean;
  }) => (
    <section
      data-testid="mcp-tool-content-block"
      data-default-expand={String(defaultExpand)}
    >
      <h2>{header}</h2>
      {actionButton}
      {children}
    </section>
  ),
}));

vi.mock(
  '../src/modes/single-mode/section-area/agent-config-area/mcp-area/mcp-selector-modal',
  () => ({
    ['McpSelectorModal']: ({
      selectedServerIds,
      onCancel,
      onConfirm,
    }: {
      selectedServerIds: string[];
      onCancel: () => void;
      onConfirm: (serverIds: string[]) => void;
    }) => (
      <div data-testid="mcp-selector-modal">
        <span>{selectedServerIds.join(',')}</span>
        <button type="button" onClick={onCancel}>
          Cancel selector
        </button>
        <button type="button" onClick={() => onConfirm(['200', '300'])}>
          Confirm selector
        </button>
      </div>
    ),
  }),
);

vi.mock('@coze-arch/coze-design/icons', () => ({
  ['IconCozLink']: () => <span data-testid="mcp-link-icon" />,
}));

vi.mock('@coze-arch/i18n', () => ({
  I18n: {
    t: (key: string) =>
      ({
        bot_edit_mcp_explain: 'Add configured MCP services to this agent.',
        bot_edit_mcp_select_title: 'Select MCP services',
        library_resource_type_mcp: 'MCP Service',
      })[key] ?? key,
  },
}));

describe('McpArea', () => {
  beforeEach(() => {
    mocks.mcpServers = [];
    mocks.updateSkillMCPServers.mockReset();
  });

  it('renders the empty state when no MCP server is selected', () => {
    render(<McpArea title="MCP Service" />);

    expect(screen.getByText('MCP Service')).toBeTruthy();
    expect(
      screen.getByText('Add configured MCP services to this agent.'),
    ).toBeTruthy();
    expect(
      screen
        .getByTestId('mcp-tool-content-block')
        .getAttribute('data-default-expand'),
    ).toBe('false');
  });

  it('renders every selected MCP server reference', () => {
    mocks.mcpServers = [{ mcp_server_id: '100' }, { mcp_server_id: '200' }];

    render(<McpArea title="MCP Service" />);

    expect(screen.getAllByTestId('bot.editor.tool.mcp.item')).toHaveLength(2);
    expect(screen.getByText('100')).toBeTruthy();
    expect(screen.getByText('200')).toBeTruthy();
    expect(
      screen
        .getByTestId('mcp-tool-content-block')
        .getAttribute('data-default-expand'),
    ).toBe('true');
  });

  it('updates the draft MCP references only after selector confirmation', () => {
    mocks.mcpServers = [{ mcp_server_id: '100' }];

    render(<McpArea title="MCP Service" />);

    fireEvent.click(screen.getByTestId('bot.editor.tool.mcp.add-button'));
    const selector = screen.getByTestId('mcp-selector-modal');
    expect(selector).toBeTruthy();
    expect(within(selector).getByText('100')).toBeTruthy();
    expect(mocks.updateSkillMCPServers).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText('Confirm selector'));
    expect(mocks.updateSkillMCPServers).toHaveBeenCalledWith([
      { mcp_server_id: '200' },
      { mcp_server_id: '300' },
    ]);
    expect(screen.queryByTestId('mcp-selector-modal')).toBeNull();
  });

  it('does not update the draft when the selector is cancelled', () => {
    render(<McpArea title="MCP Service" />);

    fireEvent.click(screen.getByTestId('bot.editor.tool.mcp.add-button'));
    fireEvent.click(screen.getByText('Cancel selector'));

    expect(mocks.updateSkillMCPServers).not.toHaveBeenCalled();
    expect(screen.queryByTestId('mcp-selector-modal')).toBeNull();
  });
});
