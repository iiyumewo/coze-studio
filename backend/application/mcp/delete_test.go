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
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	mcpmodel "github.com/coze-dev/coze-studio/backend/api/model/mcp"
	rescommon "github.com/coze-dev/coze-studio/backend/api/model/resource/common"
	mcpentity "github.com/coze-dev/coze-studio/backend/domain/mcp/entity"
	searchentity "github.com/coze-dev/coze-studio/backend/domain/search/entity"
	"github.com/coze-dev/coze-studio/backend/types/errno"
)

type fakeDeleteMCPServerRepository struct {
	server       *mcpentity.MCPServer
	getErr       error
	deleteErr    error
	getCalls     int
	deleteCalls  int
	deletedID    int64
	deletedSpace int64
}

func (f *fakeDeleteMCPServerRepository) Create(
	context.Context,
	*mcpentity.MCPServer,
) (*mcpentity.MCPServer, error) {
	return nil, errors.New("unexpected Create call")
}

func (f *fakeDeleteMCPServerRepository) GetByID(
	context.Context,
	int64,
) (*mcpentity.MCPServer, error) {
	f.getCalls++
	return f.server, f.getErr
}

func (f *fakeDeleteMCPServerRepository) Delete(
	_ context.Context,
	id,
	spaceID int64,
) error {
	f.deleteCalls++
	f.deletedID = id
	f.deletedSpace = spaceID
	return f.deleteErr
}

func TestMCPApplicationServiceDeleteMCPServer(t *testing.T) {
	request := &mcpmodel.DeleteMCPServerRequest{
		MCPServerID: testServerID,
		SpaceID:     testSpaceID,
	}

	t.Run("delete workspace MCP server and resource index", func(t *testing.T) {
		repository := &fakeDeleteMCPServerRepository{
			server: &mcpentity.MCPServer{ID: testServerID, SpaceID: testSpaceID},
		}
		eventBus := &fakeResourceEventBus{}
		service := newApplicationService(nil, repository, eventBus)

		response, err := service.DeleteMCPServer(authorizedContext(t, testSpaceID), request)

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, int64(0), response.Code)
		require.Equal(t, "success", response.Msg)
		require.NotNil(t, response.BaseResp)
		require.Equal(t, 1, repository.getCalls)
		require.Equal(t, 1, repository.deleteCalls)
		require.Equal(t, testServerID, repository.deletedID)
		require.Equal(t, testSpaceID, repository.deletedSpace)
		require.Equal(t, 1, eventBus.calls)
		require.Equal(t, searchentity.Deleted, eventBus.event.OpType)
		require.Equal(t, rescommon.ResType_MCP, eventBus.event.Resource.ResType)
		require.Equal(t, testServerID, eventBus.event.Resource.ResID)
	})

	t.Run("reject MCP server from another workspace", func(t *testing.T) {
		repository := &fakeDeleteMCPServerRepository{
			server: &mcpentity.MCPServer{ID: testServerID, SpaceID: testSpaceID + 1},
		}
		eventBus := &fakeResourceEventBus{}
		service := newApplicationService(nil, repository, eventBus)

		response, err := service.DeleteMCPServer(authorizedContext(t, testSpaceID), request)

		require.Nil(t, response)
		requireStatusErrorCode(t, err, errno.ErrMCPPermissionCode)
		require.Equal(t, 0, repository.deleteCalls)
		require.Equal(t, 0, eventBus.calls)
	})

	t.Run("reject inaccessible workspace before querying repository", func(t *testing.T) {
		repository := &fakeDeleteMCPServerRepository{}
		eventBus := &fakeResourceEventBus{}
		service := newApplicationService(nil, repository, eventBus)

		response, err := service.DeleteMCPServer(authorizedContext(t, testSpaceID+1), request)

		require.Nil(t, response)
		requireStatusErrorCode(t, err, errno.ErrMCPPermissionCode)
		require.Equal(t, 0, repository.getCalls)
		require.Equal(t, 0, repository.deleteCalls)
		require.Equal(t, 0, eventBus.calls)
	})

	t.Run("map repository delete failure", func(t *testing.T) {
		repository := &fakeDeleteMCPServerRepository{
			server:    &mcpentity.MCPServer{ID: testServerID, SpaceID: testSpaceID},
			deleteErr: errors.New("database unavailable"),
		}
		eventBus := &fakeResourceEventBus{}
		service := newApplicationService(nil, repository, eventBus)

		response, err := service.DeleteMCPServer(authorizedContext(t, testSpaceID), request)

		require.Nil(t, response)
		requireStatusErrorCode(t, err, errno.ErrMCPDeleteCode)
		require.Equal(t, 1, repository.deleteCalls)
		require.Equal(t, 0, eventBus.calls)
	})

	t.Run("map resource event failure after database delete", func(t *testing.T) {
		repository := &fakeDeleteMCPServerRepository{
			server: &mcpentity.MCPServer{ID: testServerID, SpaceID: testSpaceID},
		}
		eventBus := &fakeResourceEventBus{
			publishFn: func(context.Context, *searchentity.ResourceDomainEvent) error {
				return errors.New("event unavailable")
			},
		}
		service := newApplicationService(nil, repository, eventBus)

		response, err := service.DeleteMCPServer(authorizedContext(t, testSpaceID), request)

		require.Nil(t, response)
		requireStatusErrorCode(t, err, errno.ErrMCPDeleteCode)
		require.Equal(t, 1, repository.deleteCalls)
		require.Equal(t, 1, eventBus.calls)
	})
}
