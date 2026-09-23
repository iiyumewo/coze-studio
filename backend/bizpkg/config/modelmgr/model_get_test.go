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

package modelmgr

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	config "github.com/coze-dev/coze-studio/backend/api/model/admin/config"
	"github.com/coze-dev/coze-studio/backend/api/model/app/developer_api"
)

func TestApplyLatestModelMetaRefreshesLimitsAndKeepsIdentity(t *testing.T) {
	latest := ModelMeta{
		DisplayInfo: &config.DisplayInfo{OutputTokens: 8000, MaxTokens: 8192},
		Parameters: []*developer_api.ModelParameter{{
			Name: "max_tokens",
			Max:  "8000",
			DefaultVal: &developer_api.ModelParamDefaultValue{
				DefaultVal: "8000",
			},
		}},
	}
	modelConfig := &ModelConfig{ModelMetaConf: &ModelMetaConf{Provider2Models: map[string]map[string]ModelMeta{
		developer_api.ModelClass_QWen.String(): {"default": latest},
	}}}
	model := &Model{Model: &config.Model{
		ID: 7,
		Provider: &config.ModelProvider{
			ModelClass: developer_api.ModelClass_QWen,
		},
		DisplayInfo: &config.DisplayInfo{Name: "Qwen 3.8 Max", OutputTokens: 4096, MaxTokens: 8192},
		Connection: &config.Connection{BaseConnInfo: &config.BaseConnectionInfo{
			Model: "qwen3.8-max",
		}},
	}}

	modelConfig.applyLatestModelMeta(context.Background(), model)

	require.Equal(t, "Qwen 3.8 Max", model.DisplayInfo.Name)
	require.Equal(t, int64(8000), model.DisplayInfo.OutputTokens)
	require.Equal(t, "8000", model.Parameters[0].Max)
	require.Equal(t, "8000", model.Parameters[0].DefaultVal.DefaultVal)
}
