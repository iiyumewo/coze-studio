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
import { useBotSkillStore } from '@coze-studio/bot-detail-store/bot-skill';
import { I18n } from '@coze-arch/i18n';
import { IconCozSkill } from '@coze-arch/coze-design/icons';
import { useSpaceStore } from '@coze-arch/bot-studio-store';
import { AddButton, ToolContentBlock } from '@coze-agent-ide/tool';

import {
  fetchAvailableSkills,
  SkillSelectorModal,
} from './skill-selector-modal';

interface SkillAreaProps {
  title: string;
}

export const SkillArea: FC<SkillAreaProps> = ({ title }) => {
  const skills = useBotSkillStore(state => state.skills);
  const updateSkillSkills = useBotSkillStore(state => state.updateSkillSkills);
  const spaceId = useSpaceStore(state => state.space.id) ?? '';
  const [selectorVisible, setSelectorVisible] = useState(false);
  const { data: availableSkills = [] } = useRequest(
    () => fetchAvailableSkills(spaceId),
    { ready: Boolean(spaceId), onError: () => undefined },
  );
  const availableSkillMap = new Map(
    availableSkills.map(skill => [skill.res_id, skill]),
  );

  return (
    <>
      <ToolContentBlock
        header={title}
        defaultExpand={skills.length > 0}
        actionButton={
          <AddButton
            tooltips={I18n.t('bot_edit_skill_select_title')}
            onClick={() => setSelectorVisible(true)}
            enableAutoHidden={true}
            data-testid="bot.editor.tool.skill.add-button"
          />
        }
      >
        <div className="px-[8px]">
          {skills.length ? (
            <div className="flex flex-col gap-[8px]">
              {skills.map(skill => {
                const detail = availableSkillMap.get(skill.skill_id);
                const name = skill.name || detail?.name || skill.skill_id;
                const description =
                  skill.description || detail?.desc || skill.skill_id;
                const entryCount =
                  skill.entry_count ??
                  (detail?.biz_extend?.entry_count
                    ? Number(detail.biz_extend.entry_count)
                    : undefined);
                return (
                  <div
                    key={skill.skill_id}
                    data-testid="bot.editor.tool.skill.item"
                    className="flex min-h-[48px] items-center gap-[12px] rounded-[6px] border border-solid coz-stroke-primary px-[12px] py-[8px]"
                  >
                    <div className="flex h-[32px] w-[32px] flex-none items-center justify-center rounded-[6px] coz-mg-hglt">
                      <IconCozSkill className="h-[18px] w-[18px] coz-fg-hglt" />
                    </div>
                    <div className="min-w-0">
                      <div className="text-sm font-medium leading-[20px] coz-fg-primary">
                        {name}
                      </div>
                      <div className="truncate text-xs leading-[18px] coz-fg-secondary">
                        {description}
                        {entryCount === undefined
                          ? null
                          : ` · ${I18n.t('library_skill_files', {
                              count: entryCount,
                            })}`}
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="text-sm leading-[22px] coz-fg-secondary">
              {I18n.t('bot_edit_skill_explain')}
            </div>
          )}
        </div>
      </ToolContentBlock>
      {selectorVisible ? (
        <SkillSelectorModal
          selectedSkillIds={skills.map(skill => skill.skill_id)}
          onCancel={() => setSelectorVisible(false)}
          onConfirm={(selectedSkills: SkillItem[]) => {
            updateSkillSkills(selectedSkills);
            setSelectorVisible(false);
          }}
        />
      ) : null}
    </>
  );
};
