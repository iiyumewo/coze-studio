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

import { type FC, type ReactNode } from 'react';

import { I18n } from '@coze-arch/i18n';
import { IconCozSkill } from '@coze-arch/coze-design/icons';
import { Modal, Tag } from '@coze-arch/coze-design';
import { formatBytes, formatDate } from '@coze-arch/bot-utils';
import {
  PublishStatus,
  type ResourceInfo,
} from '@coze-arch/bot-api/plugin_develop';

interface SkillDetailModalProps {
  resourceInfo: ResourceInfo;
  onCancel: () => void;
}

interface DetailFieldProps {
  label: ReactNode;
  value: ReactNode;
  fullWidth?: boolean;
}

const DetailField: FC<DetailFieldProps> = ({ label, value, fullWidth }) => (
  <div className={fullWidth ? 'col-span-full min-w-0' : 'min-w-0'}>
    <dt className="mb-[4px] text-[12px] coz-fg-secondary">{label}</dt>
    <dd className="m-0 min-h-[20px] break-words text-[14px] coz-fg-primary">
      {value}
    </dd>
  </div>
);

const getPublishStatusLabel = (status?: PublishStatus) => {
  if (status === PublishStatus.Published) {
    return I18n.t('library_filter_tags_published');
  }
  if (status === PublishStatus.UnPublished) {
    return I18n.t('library_filter_tags_unpublished');
  }
  return '-';
};

const getFileSizeLabel = (value?: string) => {
  const fileSize = Number(value);
  return Number.isFinite(fileSize) && fileSize >= 0
    ? formatBytes(fileSize)
    : '-';
};

const getEntryCountLabel = (value?: string) => {
  const entryCount = Number(value);
  return Number.isInteger(entryCount) && entryCount >= 0
    ? I18n.t('library_skill_files', { count: entryCount })
    : '-';
};

const getEditedTimeLabel = (value?: string | number) => {
  const timestamp = Number(value);
  return Number.isFinite(timestamp) && timestamp > 0
    ? formatDate(timestamp, 'YYYY-MM-DD HH:mm')
    : '-';
};

export const SkillDetailModal: FC<SkillDetailModalProps> = ({
  resourceInfo,
  onCancel,
}) => {
  const bizExtend = resourceInfo.biz_extend ?? {};

  return (
    <Modal
      title={I18n.t('library_skill_detail_title')}
      visible={true}
      width={560}
      centered
      maskClosable={false}
      footer={null}
      onCancel={onCancel}
    >
      <div
        data-testid="workspace.library.skill.detail.content"
        className="max-h-[70vh] overflow-y-auto pr-[4px]"
      >
        <section className="flex items-start gap-[12px] pb-[20px]">
          <div className="flex h-[40px] w-[40px] shrink-0 items-center justify-center rounded-[8px] coz-mg-hglt">
            <IconCozSkill className="h-[20px] w-[20px] coz-fg-hglt" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-[8px]">
              <h3 className="m-0 break-words text-[18px] font-[600] coz-fg-primary">
                {resourceInfo.name || '-'}
              </h3>
              {resourceInfo.publish_status !== undefined ? (
                <Tag
                  color={
                    resourceInfo.publish_status === PublishStatus.Published
                      ? 'green'
                      : 'grey'
                  }
                  size="mini"
                >
                  {getPublishStatusLabel(resourceInfo.publish_status)}
                </Tag>
              ) : null}
            </div>
            {resourceInfo.desc ? (
              <p className="mb-0 mt-[6px] whitespace-pre-wrap break-words text-[13px] coz-fg-secondary">
                {resourceInfo.desc}
              </p>
            ) : null}
          </div>
        </section>

        <section className="border-0 border-t border-solid coz-stroke-primary py-[20px]">
          <h4 className="mb-[16px] mt-0 text-[14px] font-[600] coz-fg-primary">
            {I18n.t('library_skill_archive_info')}
          </h4>
          <dl className="m-0 grid grid-cols-1 gap-x-[24px] gap-y-[16px] sm:grid-cols-2">
            <DetailField
              fullWidth
              label={I18n.t('library_skill_file_name')}
              value={
                <span className="break-all font-mono text-[13px]">
                  {bizExtend.file_name || '-'}
                </span>
              }
            />
            <DetailField
              label={I18n.t('library_skill_file_size')}
              value={getFileSizeLabel(bizExtend.file_size)}
            />
            <DetailField
              label={I18n.t('library_skill_file_count')}
              value={getEntryCountLabel(bizExtend.entry_count)}
            />
          </dl>
        </section>

        <section className="grid grid-cols-1 gap-x-[24px] gap-y-[16px] border-0 border-t border-solid coz-stroke-primary pt-[20px] sm:grid-cols-2">
          <DetailField
            label={I18n.t('library_skill_resource_id')}
            value={resourceInfo.res_id || '-'}
          />
          <DetailField
            label={I18n.t('library_edited_time', {}, 'Edited time')}
            value={getEditedTimeLabel(resourceInfo.edit_time)}
          />
        </section>
      </div>
    </Modal>
  );
};
