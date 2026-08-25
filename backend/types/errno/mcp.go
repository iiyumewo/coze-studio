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

package errno

import "github.com/coze-dev/coze-studio/backend/pkg/errorx/code"

// MCP: 113 000 000 ~ 113 999 999
const (
	ErrMCPInvalidParamCode = 113000000
	ErrMCPPermissionCode   = 113000001
	ErrMCPValidateCode     = 113000002
	ErrMCPCreateCode       = 113000003
)

func init() {
	code.Register(
		ErrMCPInvalidParamCode,
		"invalid parameter : {msg}",
		code.WithAffectStability(false),
	)

	code.Register(
		ErrMCPPermissionCode,
		"unauthorized access : {msg}",
		code.WithAffectStability(false),
	)

	code.Register(
		ErrMCPValidateCode,
		"MCP server validation failed : {msg}",
		code.WithAffectStability(false),
	)

	code.Register(
		ErrMCPCreateCode,
		"create MCP server failed",
		code.WithAffectStability(true),
	)
}
