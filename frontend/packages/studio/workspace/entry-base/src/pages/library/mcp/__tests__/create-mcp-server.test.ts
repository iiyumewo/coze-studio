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
  createMcpServer,
  isValidMcpServerUrl,
  McpAuthType,
} from '../create-mcp-server';

const mocks = vi.hoisted(() => ({
  testAndCreate: vi.fn(),
}));

vi.mock('@coze-arch/bot-api', () => ({
  mcpApi: {
    testAndCreate: mocks.testAndCreate,
  },
}));

describe('createMcpServer', () => {
  beforeEach(() => {
    mocks.testAndCreate.mockReset();
    mocks.testAndCreate.mockResolvedValue({
      code: 0,
      data: {
        id: 'mcp-1',
        name: 'Example MCP',
        tool_count: 3,
      },
    });
  });

  it('accepts only HTTP and HTTPS server URLs', () => {
    expect(isValidMcpServerUrl(' https://example.com/mcp ')).toBe(true);
    expect(isValidMcpServerUrl('http://127.0.0.1:3000/mcp')).toBe(true);
    expect(isValidMcpServerUrl('ftp://example.com/mcp')).toBe(false);
    expect(isValidMcpServerUrl('example.com/mcp')).toBe(false);
  });

  it('builds a streamable HTTP request without authentication', async () => {
    await expect(
      createMcpServer('space-1', {
        serverUrl: ' https://example.com/mcp ',
        name: ' Example ',
        authType: McpAuthType.None,
      }),
    ).resolves.toEqual({
      id: 'mcp-1',
      name: 'Example MCP',
      toolCount: 3,
    });

    expect(mocks.testAndCreate).toHaveBeenCalledWith(
      {
        space_id: 'space-1',
        name: 'Example',
        server_url: 'https://example.com/mcp',
        transport: 'streamable_http',
        authentication: { type: 'none' },
      },
      { __disableErrorToast: true },
    );
  });

  it('includes a custom authentication header', async () => {
    await createMcpServer('space-1', {
      serverUrl: 'https://example.com/mcp',
      authType: McpAuthType.Header,
      headerName: ' Authorization ',
      headerValue: ' Bearer secret ',
    });

    expect(mocks.testAndCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        authentication: {
          type: 'header',
          header_name: 'Authorization',
          header_value: 'Bearer secret',
        },
      }),
      { __disableErrorToast: true },
    );
  });

  it('rejects an incomplete backend response', async () => {
    mocks.testAndCreate.mockResolvedValue({ code: 0, data: {} });

    await expect(
      createMcpServer('space-1', {
        serverUrl: 'https://example.com/mcp',
        authType: McpAuthType.None,
      }),
    ).rejects.toThrow('Invalid MCP server response');
  });
});
