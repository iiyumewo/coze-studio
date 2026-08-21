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

import { type FC, useRef, useState } from 'react';

import { I18n } from '@coze-arch/i18n';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Form, type FormApi, Modal, Toast } from '@coze-arch/coze-design';

import {
  createMcpServer,
  isMcpBackendUnavailableError,
  isValidMcpServerUrl,
  McpAuthType,
  type McpServerFormValues,
} from './create-mcp-server';

interface CreateMcpServerModalProps {
  spaceId: string;
  onCancel: () => void;
  onSuccess: () => void;
}

const getSubmitErrorMessage = (error: unknown): string => {
  if (isMcpBackendUnavailableError(error)) {
    return I18n.t('library_mcp_backend_unavailable');
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return I18n.t('library_mcp_connection_failed');
};

export const CreateMcpServerModal: FC<CreateMcpServerModalProps> = ({
  spaceId,
  onCancel,
  onSuccess,
}) => {
  const formApi = useRef<FormApi<McpServerFormValues>>();
  const [authType, setAuthType] = useState(McpAuthType.None);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState('');

  const handleSubmit = async () => {
    setSubmitError('');
    let values: McpServerFormValues | undefined;
    try {
      values = await formApi.current?.validate();
    } catch (validationError) {
      void validationError;
      return;
    }
    if (!values) {
      return;
    }

    try {
      setSubmitting(true);
      const result = await createMcpServer(spaceId, values);
      Toast.success({
        content: I18n.t('library_mcp_create_success', {
          tool_count: result.toolCount,
        }),
        showClose: false,
      });
      onSuccess();
    } catch (error) {
      setSubmitError(getSubmitErrorMessage(error));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={I18n.t('library_mcp_create_title')}
      visible={true}
      width={520}
      centered
      maskClosable={false}
      cancelText={I18n.t('Cancel')}
      okText={I18n.t('library_mcp_test_and_add')}
      okButtonProps={{ loading: submitting }}
      onCancel={onCancel}
      onOk={handleSubmit}
    >
      <Form<McpServerFormValues>
        showValidateIcon={false}
        getFormApi={api => (formApi.current = api)}
      >
        <Form.Input
          field="serverUrl"
          label={I18n.t('library_mcp_server_url')}
          placeholder={I18n.t('library_mcp_server_url_placeholder')}
          maxLength={2048}
          trigger={['blur', 'change']}
          rules={[
            {
              required: true,
              message: I18n.t('library_mcp_server_url_required'),
            },
            {
              validator: (_rule, value) =>
                !value?.trim() || isValidMcpServerUrl(value),
              message: I18n.t('library_mcp_invalid_url'),
            },
          ]}
        />
        <Form.Input
          field="name"
          label={I18n.t('library_mcp_server_name')}
          placeholder={I18n.t('library_mcp_server_name_placeholder')}
          maxLength={50}
        />
        <Form.RadioGroup
          field="authType"
          label={I18n.t('library_mcp_auth_type')}
          initValue={McpAuthType.None}
          options={[
            {
              label: I18n.t('library_mcp_auth_none'),
              value: McpAuthType.None,
            },
            {
              label: I18n.t('library_mcp_auth_header'),
              value: McpAuthType.Header,
            },
          ]}
          onChange={event => {
            setAuthType(event.target.value as McpAuthType);
            setSubmitError('');
          }}
        />
        {authType === McpAuthType.Header ? (
          <div className="grid grid-cols-2 gap-[12px]">
            <Form.Input
              field="headerName"
              label={I18n.t('library_mcp_header_name')}
              placeholder="Authorization"
              initValue="Authorization"
              maxLength={128}
              rules={[
                {
                  required: true,
                  message: I18n.t('library_mcp_header_name_required'),
                },
              ]}
            />
            <Form.Input
              field="headerValue"
              label={I18n.t('library_mcp_header_value')}
              placeholder="Bearer ..."
              type="password"
              maxLength={2048}
              rules={[
                {
                  required: true,
                  message: I18n.t('library_mcp_header_value_required'),
                },
              ]}
            />
          </div>
        ) : null}
        <div className="flex items-start gap-[8px] rounded-[6px] coz-bg-secondary px-[12px] py-[10px] text-[12px] coz-fg-secondary">
          <IconCozInfoCircle className="mt-[1px] shrink-0 text-[14px]" />
          <span>{I18n.t('library_mcp_shared_credential')}</span>
        </div>
        {submitError ? (
          <div role="alert" className="mt-[12px] text-[12px] coz-fg-hglt-red">
            {submitError}
          </div>
        ) : null}
      </Form>
    </Modal>
  );
};
