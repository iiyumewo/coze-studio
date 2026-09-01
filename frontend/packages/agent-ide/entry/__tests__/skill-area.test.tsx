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
import { fireEvent, render, screen } from '@testing-library/react';

import { SkillArea } from '../src/modes/single-mode/section-area/agent-config-area/skill-area';

const mocks = vi.hoisted(() => ({
  skills: [] as Array<{
    skill_id: string;
    name?: string;
    description?: string;
    entry_count?: number;
  }>,
  updateSkillSkills: vi.fn(),
  fetchAvailableSkills: vi.fn(),
}));

vi.mock('@coze-studio/bot-detail-store/bot-skill', () => ({
  useBotSkillStore: (selector: (state: unknown) => unknown) =>
    selector({
      skills: mocks.skills,
      updateSkillSkills: mocks.updateSkillSkills,
    }),
}));

vi.mock('@coze-arch/bot-studio-store', () => ({
  useSpaceStore: (selector: (state: { space: { id: string } }) => unknown) =>
    selector({ space: { id: 'space-1' } }),
}));

vi.mock('@coze-arch/coze-design/icons', () => ({
  ['IconCozSkill']: () => <span data-testid="skill-icon" />,
}));

vi.mock('@coze-agent-ide/tool', () => ({
  ['AddButton']: ({ onClick }: { onClick: () => void }) => (
    <button type="button" onClick={onClick}>
      Add Skill
    </button>
  ),
  ['ToolContentBlock']: ({
    header,
    children,
    actionButton,
  }: {
    header: React.ReactNode;
    children: React.ReactNode;
    actionButton?: React.ReactNode;
  }) => (
    <section>
      <h2>{header}</h2>
      {actionButton}
      {children}
    </section>
  ),
}));

vi.mock(
  '../src/modes/single-mode/section-area/agent-config-area/skill-area/skill-selector-modal',
  () => ({
    ['fetchAvailableSkills']: mocks.fetchAvailableSkills,
    ['SkillSelectorModal']: ({
      onCancel,
      onConfirm,
    }: {
      onCancel: () => void;
      onConfirm: (skills: typeof mocks.skills) => void;
    }) => (
      <div data-testid="skill-selector-modal">
        <button type="button" onClick={onCancel}>
          Cancel selector
        </button>
        <button
          type="button"
          onClick={() => onConfirm([{ skill_id: '200', name: 'Time Skill' }])}
        >
          Confirm selector
        </button>
      </div>
    ),
  }),
);

vi.mock('@coze-arch/i18n', () => ({
  I18n: {
    t: (key: string, variables?: { count?: number }) =>
      ({
        bot_edit_skill_explain: 'Add Skills to this agent.',
        bot_edit_skill_select_title: 'Select Skills',
        library_skill_files: `${variables?.count} files`,
      })[key] ?? key,
  },
}));

describe('SkillArea', () => {
  beforeEach(() => {
    mocks.skills = [];
    mocks.updateSkillSkills.mockReset();
    mocks.fetchAvailableSkills.mockResolvedValue([]);
  });

  it('renders selected Skills and their metadata', () => {
    mocks.skills = [
      {
        skill_id: '100',
        name: 'Filesystem Skill',
        description: 'writes files',
        entry_count: 2,
      },
    ];

    render(<SkillArea title="Skill" />);

    expect(screen.getByText('Filesystem Skill')).toBeTruthy();
    expect(screen.getByText('writes files · 2 files')).toBeTruthy();
  });

  it('updates the selected Skills after confirmation', () => {
    render(<SkillArea title="Skill" />);
    fireEvent.click(screen.getByText('Add Skill'));
    fireEvent.click(screen.getByText('Confirm selector'));

    expect(mocks.updateSkillSkills).toHaveBeenCalledWith([
      { skill_id: '200', name: 'Time Skill' },
    ]);
    expect(screen.queryByTestId('skill-selector-modal')).toBeNull();
  });
});
