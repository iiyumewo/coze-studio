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

export interface SkillData {
  id: string;
  name: string;
  description: string;
  file_name: string;
  file_size: number;
  entry_count: number;
}

export interface CreateSkillResponse {
  code: number;
  msg?: string;
  data?: SkillData;
}

export interface DeleteSkillResponse {
  code: number;
  msg?: string;
}

export const skillApi = {
  create: (spaceId: string, file: File, config: BotAPIRequestConfig = {}) => {
    const formData = new FormData();
    formData.append('space_id', spaceId);
    formData.append('file', file);

    return axiosInstance.request<CreateSkillResponse, CreateSkillResponse>({
      ...config,
      url: '/api/skill_manage/v1/skills',
      method: 'POST',
      data: formData,
      headers: {
        'Content-Type': 'multipart/form-data',
      },
    });
  },
  deleteSkill: (
    skillId: string,
    spaceId: string,
    config: BotAPIRequestConfig = {},
  ) =>
    axiosInstance.request<DeleteSkillResponse, DeleteSkillResponse>({
      ...config,
      url: `/api/skill_manage/v1/skills/${encodeURIComponent(skillId)}`,
      method: 'DELETE',
      params: { space_id: spaceId },
    }),
};
