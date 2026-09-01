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

import { useState } from 'react';

import { I18n } from '@coze-arch/i18n';
import { IconCozSkill } from '@coze-arch/coze-design/icons';
import { Menu, Table, Toast } from '@coze-arch/coze-design';
import {
  ActionKey,
  ResType,
  type ResourceInfo,
} from '@coze-arch/bot-api/plugin_develop';
import { skillApi } from '@coze-arch/bot-api';

import { SkillLibraryItem } from '../../skill/skill-library-item';
import { SkillDetailModal } from '../../skill/skill-detail-modal';
import { CreateSkillModal } from '../../skill/create-skill-modal';
import { type UseEntityConfigHook } from './types';

const { TableAction } = Table;

export const useSkillConfig: UseEntityConfigHook = ({
  spaceId,
  reloadList,
  getCommonActions,
}) => {
  const [visible, setVisible] = useState(false);
  const [selectedSkill, setSelectedSkill] = useState<{
    spaceId: string;
    resourceInfo: ResourceInfo;
  }>();

  const detailVisible = selectedSkill?.spaceId === spaceId;

  return {
    modals: (
      <>
        {visible ? (
          <CreateSkillModal
            spaceId={spaceId}
            onCancel={() => setVisible(false)}
            onSuccess={() => {
              setVisible(false);
              reloadList();
            }}
          />
        ) : null}
        {detailVisible ? (
          <SkillDetailModal
            key={`${selectedSkill.spaceId}-${selectedSkill.resourceInfo.res_id}`}
            resourceInfo={selectedSkill.resourceInfo}
            onCancel={() => setSelectedSkill(undefined)}
          />
        ) : null}
      </>
    ),
    config: {
      typeFilter: {
        label: I18n.t('library_resource_type_skill'),
        value: ResType.Skill,
      },
      renderCreateMenu: () => (
        <Menu.Item
          data-testid="workspace.library.header.create.skill"
          icon={<IconCozSkill />}
          onClick={() => setVisible(true)}
        >
          {I18n.t('library_resource_type_skill')}
        </Menu.Item>
      ),
      target: [ResType.Skill],
      onItemClick: item => {
        if (!item.res_id) {
          return;
        }
        setSelectedSkill({ spaceId, resourceInfo: item });
      },
      renderItem: item => <SkillLibraryItem resourceInfo={item} />,
      renderActions: (item: ResourceInfo) => {
        const deleteDisabled =
          !item.res_id ||
          !item.actions?.find(action => action.key === ActionKey.Delete)
            ?.enable;

        return (
          <TableAction
            deleteProps={{
              disabled: deleteDisabled,
              deleteDesc: I18n.t('library_skill_delete_desc'),
              handler: async () => {
                if (!item.res_id) {
                  return;
                }
                await skillApi.deleteSkill(item.res_id, spaceId);
                setSelectedSkill(current =>
                  current?.resourceInfo.res_id === item.res_id
                    ? undefined
                    : current,
                );
                reloadList();
                Toast.success(I18n.t('library_skill_delete_success'));
              },
            }}
            actionList={getCommonActions?.(item)}
          />
        );
      },
    },
  };
};
