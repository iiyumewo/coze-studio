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

import { I18n } from '@coze-arch/i18n';
import { IconCozSkill } from '@coze-arch/coze-design/icons';
import { Tag } from '@coze-arch/coze-design';
import { type ResourceInfo } from '@coze-arch/bot-api/plugin_develop';

import { BaseLibraryItem } from '../components/base-library-item';

export const SkillLibraryItem: React.FC<{ resourceInfo: ResourceInfo }> = ({
  resourceInfo,
}) => {
  const entryCount = Number(resourceInfo.biz_extend?.entry_count);
  const hasEntryCount = Number.isInteger(entryCount) && entryCount >= 0;

  return (
    <BaseLibraryItem
      resourceInfo={resourceInfo}
      customAvatar={
        <div className="mr-[12px] flex h-[40px] w-[40px] flex-shrink-0 items-center justify-center rounded-[12px] coz-mg-hglt">
          <IconCozSkill className="h-[20px] w-[20px] coz-fg-hglt" />
        </div>
      }
      tag={
        hasEntryCount ? (
          <Tag color="brand" size="mini" className="flex-shrink-0">
            {I18n.t('library_skill_files', { count: entryCount })}
          </Tag>
        ) : null
      }
    />
  );
};
