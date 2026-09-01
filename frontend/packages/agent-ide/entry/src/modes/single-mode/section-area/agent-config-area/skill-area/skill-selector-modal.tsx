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

import { type FC, useState } from 'react';

import { useRequest } from 'ahooks';
import { type SkillItem } from '@coze-studio/bot-detail-store/skill-types';
import { I18n } from '@coze-arch/i18n';
import { IconCozEmpty, IconCozSkill } from '@coze-arch/coze-design/icons';
import {
  Button,
  Checkbox,
  EmptyState,
  Modal,
  Spin,
} from '@coze-arch/coze-design';
import { useSpaceStore } from '@coze-arch/bot-studio-store';
import { ResType, type ResourceInfo } from '@coze-arch/bot-api/plugin_develop';
import { PluginDevelopApi } from '@coze-arch/bot-api';

interface SkillSelectorModalProps {
  selectedSkillIds: string[];
  onCancel: () => void;
  onConfirm: (skills: SkillItem[]) => void;
}

export type AvailableSkill = ResourceInfo & { res_id: string };

const SKILL_PAGE_SIZE = 100;

const isAvailableSkill = (resource: ResourceInfo): resource is AvailableSkill =>
  typeof resource.res_id === 'string' &&
  resource.res_id.length > 0 &&
  resource.del_flag !== true;

export const fetchAvailableSkills = async (
  spaceId: string,
  cursor?: string,
  visitedCursors = new Set<string>(),
): Promise<AvailableSkill[]> => {
  const response = await PluginDevelopApi.LibraryResourceList({
    space_id: spaceId,
    res_type_filter: [ResType.Skill],
    size: SKILL_PAGE_SIZE,
    cursor,
  });
  const resources = (response.resource_list ?? []).filter(isAvailableSkill);
  const nextCursor = response.cursor;

  if (
    !response.has_more ||
    !nextCursor ||
    nextCursor === cursor ||
    visitedCursors.has(nextCursor)
  ) {
    return resources;
  }

  visitedCursors.add(nextCursor);
  return [
    ...resources,
    ...(await fetchAvailableSkills(spaceId, nextCursor, visitedCursors)),
  ];
};

const parseEntryCount = (value?: string) => {
  if (!value?.trim()) {
    return undefined;
  }

  const entryCount = Number(value);
  return Number.isInteger(entryCount) && entryCount >= 0
    ? entryCount
    : undefined;
};

const resourceToSkill = (resource: AvailableSkill): SkillItem => ({
  skill_id: resource.res_id,
  name: resource.name,
  description: resource.desc,
  entry_count: parseEntryCount(resource.biz_extend?.entry_count),
});

export const SkillSelectorModal: FC<SkillSelectorModalProps> = ({
  selectedSkillIds,
  onCancel,
  onConfirm,
}) => {
  const spaceId = useSpaceStore(state => state.space.id) ?? '';
  const [pendingSkillIds, setPendingSkillIds] = useState(selectedSkillIds);
  const {
    data: availableSkills = [],
    loading,
    error,
    refresh,
  } = useRequest(() => fetchAvailableSkills(spaceId), {
    ready: Boolean(spaceId),
    onError: () => undefined,
    onSuccess: skills => {
      const availableSkillIds = new Set(skills.map(skill => skill.res_id));
      setPendingSkillIds(
        selectedSkillIds.filter(skillId => availableSkillIds.has(skillId)),
      );
    },
  });

  const handleSkillChange = (skillId: string, checked: boolean) => {
    setPendingSkillIds(currentIds =>
      checked
        ? [...new Set([...currentIds, skillId])]
        : currentIds.filter(currentId => currentId !== skillId),
    );
  };

  return (
    <Modal
      title={I18n.t('bot_edit_skill_select_title')}
      visible={true}
      width={520}
      centered
      maskClosable={false}
      cancelText={I18n.t('Cancel')}
      okText={I18n.t('Confirm')}
      okButtonProps={{ disabled: loading || Boolean(error) || !spaceId }}
      onCancel={onCancel}
      onOk={() =>
        onConfirm(
          availableSkills
            .filter(skill => pendingSkillIds.includes(skill.res_id))
            .map(resourceToSkill),
        )
      }
    >
      {loading ? (
        <div
          data-testid="bot.editor.tool.skill.selector.loading"
          className="flex h-[240px] items-center justify-center"
        >
          <Spin />
        </div>
      ) : error || !spaceId ? (
        <div className="flex h-[240px] flex-col items-center justify-center gap-[12px]">
          <div role="alert" className="text-sm coz-fg-secondary">
            {I18n.t('bot_edit_skill_select_load_failed')}
          </div>
          {spaceId ? (
            <Button size="small" color="primary" onClick={refresh}>
              {I18n.t('Retry')}
            </Button>
          ) : null}
        </div>
      ) : availableSkills.length ? (
        <div className="flex max-h-[360px] flex-col gap-[8px] overflow-y-auto pr-[4px]">
          {availableSkills.map(skill => (
            <Checkbox
              key={skill.res_id}
              checked={pendingSkillIds.includes(skill.res_id)}
              onChange={event =>
                handleSkillChange(skill.res_id, Boolean(event.target.checked))
              }
              className="w-full rounded-[6px] border border-solid coz-stroke-primary px-[12px] py-[10px]"
            >
              <div className="flex min-w-0 items-center gap-[12px]">
                <div className="flex h-[36px] w-[36px] flex-none items-center justify-center rounded-[6px] coz-mg-hglt">
                  <IconCozSkill className="h-[18px] w-[18px] coz-fg-hglt" />
                </div>
                <div className="min-w-0">
                  <div className="truncate text-sm font-medium leading-[20px] coz-fg-primary">
                    {skill.name || skill.res_id}
                  </div>
                  <div className="truncate text-xs leading-[18px] coz-fg-secondary">
                    {skill.desc || skill.res_id}
                  </div>
                </div>
              </div>
            </Checkbox>
          ))}
        </div>
      ) : (
        <EmptyState
          className="my-[72px] mx-auto"
          icon={<IconCozEmpty />}
          title={I18n.t('bot_edit_skill_select_empty')}
        />
      )}
    </Modal>
  );
};
