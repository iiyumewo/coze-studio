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

import { axiosInstance, type BotAPIRequestConfig } from './axios';

export type McpServerAuthentication =
  | { type: 'none' }
  | {
      type: 'header';
      header_name: string;
      header_value: string;
    };

export interface TestAndCreateMcpServerRequest {
  space_id: string;
  name?: string;
  server_url: string;
  transport: 'streamable_http';
  authentication: McpServerAuthentication;
}

export interface TestAndCreateMcpServerResponse {
  code: number;
  msg?: string;
  data?: {
    id?: string;
    name?: string;
    tool_count?: number;
  };
}

export interface McpServerDetailAuthentication {
  type: 'none' | 'header';
  header_name?: string;
}

export interface McpServerTool {
  name: string;
  title?: string;
  description?: string;
}

export interface McpServerDetail {
  id: string;
  name: string;
  description?: string;
  server_url: string;
  transport: 'streamable_http';
  authentication: McpServerDetailAuthentication;
  tools: McpServerTool[];
  protocol_version?: string;
  server_version?: string;
  status: number;
  validation_status: string;
  last_error?: string;
  last_checked_at: number;
  created_at: number;
  updated_at: number;
}

export interface GetMcpServerResponse {
  code: number;
  msg?: string;
  data?: McpServerDetail;
}

export interface DeleteMcpServerResponse {
  code: number;
  msg?: string;
}

export const mcpApi = {
  testAndCreate: (
    request: TestAndCreateMcpServerRequest,
    config: BotAPIRequestConfig = {},
  ) =>
    axiosInstance.request<
      TestAndCreateMcpServerResponse,
      TestAndCreateMcpServerResponse
    >({
      ...config,
      url: '/api/mcp_manage/v1/mcp_servers/test_and_create',
      method: 'POST',
      data: request,
    }),
  getDetail: (
    mcpServerId: string,
    spaceId: string,
    config: BotAPIRequestConfig = {},
  ) =>
    axiosInstance.request<GetMcpServerResponse, GetMcpServerResponse>({
      ...config,
      url: `/api/mcp_manage/v1/mcp_servers/${encodeURIComponent(mcpServerId)}`,
      method: 'GET',
      params: { space_id: spaceId },
    }),
  deleteServer: (
    mcpServerId: string,
    spaceId: string,
    config: BotAPIRequestConfig = {},
  ) =>
    axiosInstance.request<DeleteMcpServerResponse, DeleteMcpServerResponse>({
      ...config,
      url: `/api/mcp_manage/v1/mcp_servers/${encodeURIComponent(mcpServerId)}`,
      method: 'DELETE',
      params: { space_id: spaceId },
    }),
};
