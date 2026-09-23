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

import (
	"context"
	"sync"
)

// FileSearchFunc searches files associated with the current conversation.
// The implementation owns file authorization, cache lookup and parsing.
type FileSearchFunc func(
	ctx context.Context,
	conversationID int64,
	agentID int64,
	userID string,
	fileRef string,
	query string,
) (string, error)

// FileDownloadFunc materializes a conversation file into the Skill workspace
// and returns the path relative to that workspace.
type FileDownloadFunc func(
	ctx context.Context,
	conversationID int64,
	agentID int64,
	userID string,
	fileRef string,
) (string, error)

type UploadedFile struct {
	FileKey string `json:"file_key"`
	Name    string `json:"file_name"`
	Type    string `json:"file_type"`
	Size    int64  `json:"file_size"`
	URL     string `json:"file_url"`
}

type FileUploadFunc func(
	ctx context.Context,
	conversationID int64,
	agentID int64,
	userID string,
	path string,
) (*UploadedFile, error)

type UploadedFileCollector struct {
	mu    sync.Mutex
	files []UploadedFile
}

func NewUploadedFileCollector() *UploadedFileCollector {
	return &UploadedFileCollector{}
}

func (c *UploadedFileCollector) Add(file *UploadedFile) {
	if c == nil || file == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, existing := range c.files {
		if existing.FileKey == file.FileKey {
			return
		}
	}
	c.files = append(c.files, *file)
}

func (c *UploadedFileCollector) Files() []UploadedFile {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]UploadedFile(nil), c.files...)
}

type uploadedFileCollectorContextKey struct{}

func WithUploadedFileCollector(ctx context.Context, collector *UploadedFileCollector) context.Context {
	return context.WithValue(ctx, uploadedFileCollectorContextKey{}, collector)
}

func UploadedFileCollectorFromContext(ctx context.Context) *UploadedFileCollector {
	collector, _ := ctx.Value(uploadedFileCollectorContextKey{}).(*UploadedFileCollector)
	return collector
}
