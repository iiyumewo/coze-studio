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

package skill

import (
	"context"

	"github.com/coze-dev/coze-studio/backend/application/base/ctxutil"
	crossuser "github.com/coze-dev/coze-studio/backend/crossdomain/user"
	"github.com/coze-dev/coze-studio/backend/pkg/errorx"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

func authorizeWorkspaceAccess(ctx context.Context, spaceID int64) (int64, error) {
	if spaceID <= 0 {
		return 0, errorx.New(errno.ErrSkillInvalidParamCode, errorx.KV("msg", "space_id must be greater than zero"))
	}

	session := ctxutil.GetUserSessionFromCtx(ctx)
	if session == nil || session.UserID <= 0 {
		return 0, errorx.New(errno.ErrSkillPermissionCode, errorx.KV("msg", "session is required"))
	}

	spaces, err := crossuser.DefaultSVC().GetUserSpaceList(ctx, session.UserID)
	if err != nil {
		return 0, errorx.WrapByCode(err, errno.ErrSkillPermissionCode,
			errorx.KV("msg", "failed to verify workspace access"))
	}
	for _, space := range spaces {
		if space != nil && space.ID == spaceID {
			return session.UserID, nil
		}
	}

	return 0, errorx.New(errno.ErrSkillPermissionCode, errorx.KV("msg", "no permission for workspace"))
}
