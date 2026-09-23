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

/*
 * Copyright 2025 Coze Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 */

package internal

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/eino/schema"

	crossmessage "github.com/coze-dev/coze-studio/backend/crossdomain/message"
	message "github.com/coze-dev/coze-studio/backend/crossdomain/message/model"
	"github.com/coze-dev/coze-studio/backend/domain/conversation/agentrun/entity"
	msgEntity "github.com/coze-dev/coze-studio/backend/domain/conversation/message/entity"
)

type assistantFileContent struct {
	FileList []assistantFileItem `json:"file_list"`
}

type assistantFileItem struct {
	FileKey  string `json:"file_key"`
	FileName string `json:"file_name"`
	FileType string `json:"file_type"`
	FileSize int64  `json:"file_size"`
	FileURL  string `json:"file_url"`
}

func (art *AgentRuntime) emitUploadedFiles(ctx context.Context) error {
	for _, file := range art.UploadedFiles {
		content, err := json.Marshal(assistantFileContent{FileList: []assistantFileItem{{
			FileKey:  file.FileKey,
			FileName: file.Name,
			FileType: file.Type,
			FileSize: file.Size,
			FileURL:  file.URL,
		}}})
		if err != nil {
			return err
		}
		modelContent, err := json.Marshal(&schema.Message{
			Role: schema.Assistant,
			MultiContent: []schema.ChatMessagePart{{
				Type:    schema.ChatMessagePartTypeFileURL,
				FileURL: &schema.ChatMessageFileURL{URI: file.FileKey, URL: file.URL, Name: file.Name},
			}},
		})
		if err != nil {
			return err
		}
		msg := &msgEntity.Message{
			ConversationID: art.RunMeta.ConversationID,
			RunID:          art.RunRecord.ID,
			AgentID:        art.RunMeta.AgentID,
			SectionID:      art.RunMeta.SectionID,
			UserID:         art.RunMeta.UserID,
			Role:           schema.Assistant,
			MessageType:    message.MessageTypeAnswer,
			ContentType:    message.ContentTypeFile,
			Content:        string(content),
			ModelContent:   string(modelContent),
			MultiContent: []*message.InputMetaData{{
				Type:     message.InputTypeFile,
				FileData: []*message.FileData{{Url: file.URL, URI: file.FileKey, Name: file.Name}},
			}},
		}
		created, err := crossmessage.DefaultSVC().Create(ctx, msg)
		if err != nil {
			return err
		}
		art.MessageEvent.SendMsgEvent(entity.RunEventMessageCompleted, buildSendMsg(ctx, created, true, art), art.SW)
	}
	return nil
}
