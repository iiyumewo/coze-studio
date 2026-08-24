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

package entity

import "encoding/json"

type TransportType string

const (
	TransportTypeStreamableHTTP TransportType = "streamable_http"
)

type AuthenticationType string

const (
	AuthenticationTypeNone   AuthenticationType = "none"
	AuthenticationTypeHeader AuthenticationType = "header"
)

type ServerStatus uint8

const (
	ServerStatusEnabled  ServerStatus = 1
	ServerStatusDisabled ServerStatus = 2
)

type ValidationStatus string

const (
	ValidationStatusSuccess ValidationStatus = "success"
)

type ToolAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    bool   `json:"read_only_hint,omitempty"`
	DestructiveHint *bool  `json:"destructive_hint,omitempty"`
	IdempotentHint  bool   `json:"idempotent_hint,omitempty"`
	OpenWorldHint   *bool  `json:"open_world_hint,omitempty"`
}

type Tool struct {
	Name         string           `json:"name"`
	Title        string           `json:"title,omitempty"`
	Description  string           `json:"description,omitempty"`
	InputSchema  json.RawMessage  `json:"input_schema"`
	OutputSchema json.RawMessage  `json:"output_schema,omitempty"`
	Annotations  *ToolAnnotations `json:"annotations,omitempty"`
}

type MCPServer struct {
	ID               int64
	SpaceID          int64
	CreatorID        int64
	Name             string
	Description      string
	ServerURL        string
	Transport        TransportType
	AuthType         AuthenticationType
	EncryptedAuth    *string
	Tools            []*Tool
	ProtocolVersion  string
	ServerVersion    string
	Status           ServerStatus
	ValidationStatus ValidationStatus
	LastError        string
	LastCheckedAt    int64
	CreatedAt        int64
	UpdatedAt        int64
}
