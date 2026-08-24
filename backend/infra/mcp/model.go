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

package mcp

import (
	"context"
	"encoding/json"
	"errors"
)

type AuthenticationType string

const (
	AuthenticationTypeNone   AuthenticationType = "none"
	AuthenticationTypeHeader AuthenticationType = "header"
)

var (
	ErrInvalidRequest     = errors.New("invalid MCP test request")
	ErrConnectServer      = errors.New("connect to MCP server failed")
	ErrDiscoverTools      = errors.New("discover MCP tools failed")
	ErrNormalizeTool      = errors.New("normalize MCP tool failed")
	ErrRedirectNotAllowed = errors.New("MCP server redirect is not allowed")
)

type Authentication struct {
	Type        AuthenticationType `json:"type"`
	HeaderName  string             `json:"header_name,omitempty"`
	HeaderValue string             `json:"header_value,omitempty"`
}

type TestRequest struct {
	ServerURL      string
	Authentication Authentication
}

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

type TestResult struct {
	ServerName      string
	ServerTitle     string
	ServerVersion   string
	ProtocolVersion string
	Instructions    string
	Tools           []*Tool
}

type Client interface {
	Test(ctx context.Context, request *TestRequest) (*TestResult, error)
}
