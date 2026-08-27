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

package dal

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-studio/backend/domain/mcp/entity"
	idgenmock "github.com/coze-dev/coze-studio/backend/internal/mock/infra/idgen"
)

func TestMCPServerDAOCreate(t *testing.T) {
	const serverID int64 = 12345

	sqlDB, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
		require.NoError(t, sqlMock.ExpectationsWereMet())
	})

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)

	controller := gomock.NewController(t)
	idGen := idgenmock.NewMockIDGenerator(controller)
	idGen.EXPECT().GenID(gomock.Any()).Return(serverID, nil)

	sqlMock.ExpectExec("INSERT INTO `mcp_server`").WillReturnResult(sqlmock.NewResult(1, 1))
	sqlMock.ExpectClose()

	encryptedAuth := "encrypted-credentials"
	server := &entity.MCPServer{
		SpaceID:       100,
		CreatorID:     200,
		Name:          "MySQL MCP",
		Description:   "Database tools",
		ServerURL:     "https://mcp.example.com/mcp",
		AuthType:      entity.AuthenticationTypeHeader,
		EncryptedAuth: &encryptedAuth,
		Tools: []*entity.Tool{
			{
				Name:        "query",
				Description: "Run a read-only query",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
		},
		ProtocolVersion: "2025-03-26",
		ServerVersion:   "1.0.0",
	}

	created, err := NewMCPServerDAO(db, idGen).Create(context.Background(), server)
	require.NoError(t, err)
	require.Equal(t, serverID, created.ID)
	require.Equal(t, entity.TransportTypeStreamableHTTP, created.Transport)
	require.Equal(t, entity.ServerStatusEnabled, created.Status)
	require.Equal(t, entity.ValidationStatusSuccess, created.ValidationStatus)
	require.Equal(t, encryptedAuth, *created.EncryptedAuth)
	require.Len(t, created.Tools, 1)
	require.NotZero(t, created.LastCheckedAt)
	require.Equal(t, created.CreatedAt, created.UpdatedAt)
}

func TestMCPServerDAOGetByID(t *testing.T) {
	const serverID int64 = 12345

	newDAO := func(t *testing.T) (*MCPServerDAO, sqlmock.Sqlmock) {
		t.Helper()

		sqlDB, sqlMock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, sqlDB.Close())
			require.NoError(t, sqlMock.ExpectationsWereMet())
		})

		db, err := gorm.Open(mysql.New(mysql.Config{
			Conn:                      sqlDB,
			SkipInitializeWithVersion: true,
		}), &gorm.Config{SkipDefaultTransaction: true})
		require.NoError(t, err)

		controller := gomock.NewController(t)
		idGen := idgenmock.NewMockIDGenerator(controller)
		return NewMCPServerDAO(db, idGen), sqlMock
	}

	query := regexp.QuoteMeta("SELECT * FROM `mcp_server` WHERE `mcp_server`.`id` = ? AND `mcp_server`.`deleted_at` IS NULL ORDER BY `mcp_server`.`id` LIMIT ?")

	t.Run("return MCP server", func(t *testing.T) {
		dao, sqlMock := newDAO(t)
		rows := sqlmock.NewRows([]string{
			"id", "space_id", "creator_id", "name", "description", "server_url",
			"transport", "auth_type", "encrypted_auth", "tools", "protocol_version",
			"server_version", "status", "validation_status", "last_error",
			"last_checked_at", "created_at", "updated_at", "deleted_at",
		}).AddRow(
			serverID, 100, 200, "MySQL MCP", "Database tools", "https://mcp.example.com/mcp",
			"streamable_http", "none", nil, `[{"name":"query","input_schema":{"type":"object"}}]`,
			"2025-03-26", "1.0.0", 1, "success", "", 1000, 900, 1000, nil,
		)
		sqlMock.ExpectQuery(query).WithArgs(serverID, 1).WillReturnRows(rows)
		sqlMock.ExpectClose()

		server, err := dao.GetByID(context.Background(), serverID)
		require.NoError(t, err)
		require.Equal(t, serverID, server.ID)
		require.Equal(t, int64(100), server.SpaceID)
		require.Equal(t, "MySQL MCP", server.Name)
		require.Len(t, server.Tools, 1)
		require.Equal(t, "query", server.Tools[0].Name)
	})

	t.Run("return record not found error", func(t *testing.T) {
		dao, sqlMock := newDAO(t)
		sqlMock.ExpectQuery(query).WithArgs(serverID, 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
		sqlMock.ExpectClose()

		server, err := dao.GetByID(context.Background(), serverID)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
		require.Nil(t, server)
	})
}

func TestMCPServerDAOMGetByIDs(t *testing.T) {
	const spaceID int64 = 100

	sqlDB, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
		require.NoError(t, sqlMock.ExpectationsWereMet())
	})

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)

	controller := gomock.NewController(t)
	idGen := idgenmock.NewMockIDGenerator(controller)
	dao := NewMCPServerDAO(db, idGen)

	t.Run("return empty list for empty IDs", func(t *testing.T) {
		servers, err := dao.MGetByIDs(context.Background(), spaceID, nil)
		require.NoError(t, err)
		require.Empty(t, servers)
	})

	query := regexp.QuoteMeta("SELECT * FROM `mcp_server` WHERE `mcp_server`.`space_id` = ? AND `mcp_server`.`id` IN (?,?) AND `mcp_server`.`deleted_at` IS NULL")
	rows := sqlmock.NewRows([]string{
		"id", "space_id", "name", "server_url", "transport", "auth_type", "tools",
		"status", "validation_status", "last_checked_at", "created_at", "updated_at", "deleted_at",
	}).
		AddRow(12345, spaceID, "Playwright", "http://localhost:8931/mcp", "streamable_http", "none",
			`[{"name":"browser_navigate","input_schema":{"type":"object"}}]`, 1, "success", 1000, 900, 1000, nil).
		AddRow(12346, spaceID, "MySQL", "http://localhost:8932/mcp", "streamable_http", "none",
			`[{"name":"query","input_schema":{"type":"object"}}]`, 1, "success", 1000, 900, 1000, nil)
	sqlMock.ExpectQuery(query).WithArgs(spaceID, int64(12345), int64(12346)).WillReturnRows(rows)
	sqlMock.ExpectClose()

	servers, err := dao.MGetByIDs(context.Background(), spaceID, []int64{12345, 12346})
	require.NoError(t, err)
	require.Len(t, servers, 2)
	require.Equal(t, "browser_navigate", servers[0].Tools[0].Name)
	require.Equal(t, "query", servers[1].Tools[0].Name)
}

func TestMCPServerDAODelete(t *testing.T) {
	const (
		serverID int64 = 12345
		spaceID  int64 = 100
	)

	newDAO := func(t *testing.T) (*MCPServerDAO, sqlmock.Sqlmock) {
		t.Helper()

		sqlDB, sqlMock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, sqlDB.Close())
			require.NoError(t, sqlMock.ExpectationsWereMet())
		})

		db, err := gorm.Open(mysql.New(mysql.Config{
			Conn:                      sqlDB,
			SkipInitializeWithVersion: true,
		}), &gorm.Config{SkipDefaultTransaction: true})
		require.NoError(t, err)

		controller := gomock.NewController(t)
		idGen := idgenmock.NewMockIDGenerator(controller)
		return NewMCPServerDAO(db, idGen), sqlMock
	}

	deleteQuery := regexp.QuoteMeta("UPDATE `mcp_server` SET `deleted_at`=? WHERE `mcp_server`.`id` = ? AND `mcp_server`.`space_id` = ? AND `mcp_server`.`deleted_at` IS NULL")

	t.Run("soft delete workspace MCP server", func(t *testing.T) {
		dao, sqlMock := newDAO(t)
		sqlMock.ExpectExec(deleteQuery).
			WithArgs(sqlmock.AnyArg(), serverID, spaceID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		sqlMock.ExpectClose()

		require.NoError(t, dao.Delete(context.Background(), serverID, spaceID))
	})

	t.Run("return record not found when no row is deleted", func(t *testing.T) {
		dao, sqlMock := newDAO(t)
		sqlMock.ExpectExec(deleteQuery).
			WithArgs(sqlmock.AnyArg(), serverID, spaceID).
			WillReturnResult(sqlmock.NewResult(0, 0))
		sqlMock.ExpectClose()

		require.ErrorIs(t, dao.Delete(context.Background(), serverID, spaceID), gorm.ErrRecordNotFound)
	})
}
