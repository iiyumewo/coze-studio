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
