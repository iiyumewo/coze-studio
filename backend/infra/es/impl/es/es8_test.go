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

package es

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/require"
)

func TestES8ClientCreateRefreshesIndex(t *testing.T) {
	var refresh string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		refresh = request.URL.Query().Get("refresh")
		response.Header().Set("Content-Type", "application/json")
		response.Header().Set("X-Elastic-Product", "Elasticsearch")
		response.WriteHeader(http.StatusCreated)
		_, err := response.Write([]byte(`{
			"_index":"coze_resource",
			"_id":"123",
			"_version":1,
			"result":"created",
			"_shards":{"total":1,"successful":1,"failed":0},
			"_seq_no":0,
			"_primary_term":1
		}`))
		if err != nil {
			t.Errorf("write Elasticsearch response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{server.URL},
	})
	require.NoError(t, err)

	err = (&es8Client{esClient: client, types: &es8Types{}}).Create(
		context.Background(),
		"coze_resource",
		"123",
		map[string]any{"name": "playwright"},
	)
	require.NoError(t, err)
	require.Equal(t, "true", refresh)
}
