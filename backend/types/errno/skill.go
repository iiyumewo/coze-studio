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

// Skill: 114 000 000 ~ 114 999 999
const (
	ErrSkillInvalidParamCode = 114000000
	ErrSkillPermissionCode   = 114000001
	ErrSkillValidateCode     = 114000002
	ErrSkillCreateCode       = 114000003
	ErrSkillGetCode          = 114000004
	ErrSkillDeleteCode       = 114000005
)

func init() {
	code.Register(ErrSkillInvalidParamCode, "invalid parameter : {msg}", code.WithAffectStability(false))
	code.Register(ErrSkillPermissionCode, "unauthorized access : {msg}", code.WithAffectStability(false))
	code.Register(ErrSkillValidateCode, "Skill validation failed : {msg}", code.WithAffectStability(false))
	code.Register(ErrSkillCreateCode, "create Skill failed", code.WithAffectStability(true))
	code.Register(ErrSkillGetCode, "get Skill failed", code.WithAffectStability(true))
	code.Register(ErrSkillDeleteCode, "delete Skill failed", code.WithAffectStability(true))
}
