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

import { type ChangeEvent, type FC, useRef, useState } from 'react';

import { I18n } from '@coze-arch/i18n';
import { IconCozUpload } from '@coze-arch/coze-design/icons';
import { Button, Modal, Toast } from '@coze-arch/coze-design';
import { skillApi } from '@coze-arch/bot-api';

interface CreateSkillModalProps {
  spaceId: string;
  onCancel: () => void;
  onSuccess: () => void;
}

const isSkillArchive = (file: File) => {
  const fileName = file.name.toLowerCase();
  return fileName.endsWith('.zip') || fileName.endsWith('.skill');
};

export const CreateSkillModal: FC<CreateSkillModalProps> = ({
  spaceId,
  onCancel,
  onSuccess,
}) => {
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File>();
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const selected = event.target.files?.[0];
    if (!selected) {
      return;
    }
    if (!isSkillArchive(selected)) {
      setFile(undefined);
      setError(I18n.t('library_skill_invalid_file'));
      event.target.value = '';
      return;
    }
    setError('');
    setFile(selected);
  };

  const handleSubmit = async () => {
    if (!file) {
      setError(I18n.t('library_skill_file_required'));
      return;
    }
    setError('');
    setSubmitting(true);
    try {
      const response = await skillApi.create(spaceId, file, {
        __disableErrorToast: true,
      });
      if (!response.data?.id) {
        throw new Error(response.msg || I18n.t('library_skill_upload_failed'));
      }
      Toast.success(I18n.t('library_skill_create_success'));
      onSuccess();
    } catch (submitError) {
      setError(
        submitError instanceof Error && submitError.message
          ? submitError.message
          : I18n.t('library_skill_upload_failed'),
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={I18n.t('library_skill_create_title')}
      visible={true}
      width={520}
      centered
      maskClosable={false}
      cancelText={I18n.t('Cancel')}
      okText={I18n.t('library_skill_upload')}
      okButtonProps={{ loading: submitting }}
      onCancel={onCancel}
      onOk={handleSubmit}
    >
      <div className="flex flex-col gap-[12px]">
        <div className="coz-fg-primary text-[14px]">
          {I18n.t('library_skill_file_label')}
        </div>
        <input
          ref={inputRef}
          type="file"
          accept=".zip,.skill"
          className="hidden"
          onChange={handleFileChange}
        />
        <Button
          icon={<IconCozUpload />}
          onClick={() => inputRef.current?.click()}
        >
          {I18n.t('library_skill_choose_file')}
        </Button>
        {file ? (
          <div className="coz-fg-secondary text-[13px] break-all">
            {file.name}
          </div>
        ) : null}
        <div className="coz-fg-secondary text-[12px]">
          {I18n.t('library_skill_file_hint')}
        </div>
        {error ? <div className="coz-fg-red text-[12px]">{error}</div> : null}
      </div>
    </Modal>
  );
};
