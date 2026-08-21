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

import { isAxiosError } from 'axios';
import { mcpApi } from '@coze-arch/bot-api';

export enum McpAuthType {
  None = 'none',
  Header = 'header',
}

export interface McpServerFormValues {
  serverUrl: string;
  name?: string;
  authType: McpAuthType;
  headerName?: string;
  headerValue?: string;
}

export interface CreatedMcpServer {
  id: string;
  name?: string;
  toolCount: number;
}

const HTTP_NOT_FOUND_STATUS = 404;

export const isValidMcpServerUrl = (value?: string): boolean => {
  try {
    const url = new URL(value?.trim() ?? '');
    return url.protocol === 'http:' || url.protocol === 'https:';
  } catch (error) {
    void error;
    return false;
  }
};

export const isMcpBackendUnavailableError = (error: unknown): boolean =>
  isAxiosError(error) && error.response?.status === HTTP_NOT_FOUND_STATUS;

export const createMcpServer = async (
  spaceId: string,
  values: McpServerFormValues,
): Promise<CreatedMcpServer> => {
  const response = await mcpApi.testAndCreate(
    {
      space_id: spaceId,
      name: values.name?.trim() || undefined,
      server_url: values.serverUrl.trim(),
      transport: 'streamable_http',
      authentication:
        values.authType === McpAuthType.Header
          ? {
              type: 'header',
              header_name: values.headerName?.trim() ?? '',
              header_value: values.headerValue?.trim() ?? '',
            }
          : { type: 'none' },
    },
    { __disableErrorToast: true },
  );

  if (!response.data?.id) {
    throw new Error(response.msg || 'Invalid MCP server response');
  }

  return {
    id: response.data.id,
    name: response.data.name,
    toolCount: response.data.tool_count ?? 0,
  };
};
