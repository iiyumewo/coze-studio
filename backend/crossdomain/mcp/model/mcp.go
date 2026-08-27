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

package model

import "encoding/json"

type MGetAgentToolsRequest struct {
	SpaceID      int64
	MCPServerIDs []int64
}

type ToolInfo struct {
	MCPServerID   int64
	MCPServerName string
	Name          string
	Description   string
	InputSchema   json.RawMessage
}

type ExecuteToolRequest struct {
	SpaceID     int64
	MCPServerID int64
	ToolName    string
	Arguments   json.RawMessage
}

type ExecuteToolResponse struct {
	Output  string
	IsError bool
}
