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

import { getMcpServerDetail } from '../get-mcp-server';

const mocks = vi.hoisted(() => ({
  getDetail: vi.fn(),
}));

vi.mock('@coze-arch/bot-api', () => ({
  mcpApi: {
    getDetail: mocks.getDetail,
  },
}));

describe('getMcpServerDetail', () => {
  beforeEach(() => {
    mocks.getDetail.mockReset();
    mocks.getDetail.mockResolvedValue({
      code: 0,
      data: {
        id: 'mcp-1',
        name: 'Playwright',
        server_url: 'http://127.0.0.1:8931/mcp',
        transport: 'streamable_http',
        authentication: { type: 'none' },
        tools: [],
        status: 1,
        validation_status: 'success',
        last_checked_at: 100,
        created_at: 200,
        updated_at: 300,
      },
    });
  });

  it('requests an MCP server within the current space', async () => {
    await expect(getMcpServerDetail('space-1', 'mcp-1')).resolves.toEqual(
      expect.objectContaining({ id: 'mcp-1', name: 'Playwright' }),
    );

    expect(mocks.getDetail).toHaveBeenCalledWith('mcp-1', 'space-1', {
      __disableErrorToast: true,
    });
  });

  it('rejects an incomplete backend response', async () => {
    mocks.getDetail.mockResolvedValue({ code: 0, data: {} });

    await expect(getMcpServerDetail('space-1', 'mcp-1')).rejects.toThrow(
      'Invalid MCP server detail response',
    );
  });
});
