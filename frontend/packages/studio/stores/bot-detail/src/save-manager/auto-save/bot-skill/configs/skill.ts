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

import { DebounceTime, type HostedObserverConfig } from '@coze-studio/autosave';

import { type SkillItem } from '@/types/skill';
import { type BotSkillStore } from '@/store/bot-skill';
import { ItemTypeExtra } from '@/save-manager/types';

export const skillConfig: HostedObserverConfig<
  BotSkillStore,
  ItemTypeExtra,
  SkillItem[]
> = {
  key: ItemTypeExtra.Skill,
  selector: store => store.skills,
  debounce: DebounceTime.Immediate,
  middleware: {
    onBeforeSave: skills => ({
      skill_info_list: skills.map(skill => ({
        skill_id: skill.skill_id,
      })),
    }),
  },
};
