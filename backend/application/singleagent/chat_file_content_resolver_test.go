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

package singleagent

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/require"

	documentparser "github.com/coze-dev/coze-studio/backend/infra/document/parser"
	"github.com/coze-dev/coze-studio/backend/infra/storage"
)

type fakeChatFileStorage struct {
	key  string
	body []byte
}

func (f *fakeChatFileStorage) PutObject(_ context.Context, key string, body []byte, _ ...storage.PutOptFn) error {
	f.key = key
	f.body = body
	return nil
}

func (f *fakeChatFileStorage) PutObjectWithReader(context.Context, string, io.Reader, ...storage.PutOptFn) error {
	return nil
}

func (f *fakeChatFileStorage) GetObject(_ context.Context, key string) ([]byte, error) {
	f.key = key
	return f.body, nil
}

func (f *fakeChatFileStorage) DeleteObject(context.Context, string) error { return nil }

func (f *fakeChatFileStorage) GetObjectUrl(context.Context, string, ...storage.GetOptFn) (string, error) {
	return "", nil
}

func (f *fakeChatFileStorage) HeadObject(context.Context, string, ...storage.GetOptFn) (*storage.FileInfo, error) {
	return nil, nil
}

func (f *fakeChatFileStorage) ListAllObjects(context.Context, string, ...storage.GetOptFn) ([]*storage.FileInfo, error) {
	return nil, nil
}

func (f *fakeChatFileStorage) ListObjectsPaginated(context.Context, *storage.ListObjectsPaginatedInput, ...storage.GetOptFn) (*storage.ListObjectsPaginatedOutput, error) {
	return nil, nil
}

type fakeChatFileParserManager struct {
	parser documentparser.Parser
}

func (f *fakeChatFileParserManager) GetParser(*documentparser.Config) (documentparser.Parser, error) {
	return f.parser, nil
}

func (f *fakeChatFileParserManager) IsAutoAnnotationSupported() bool { return false }

type fakeChatFileParser struct{}

func (fakeChatFileParser) Parse(context.Context, io.Reader, ...parser.Option) ([]*schema.Document, error) {
	return []*schema.Document{{Content: "parsed document content"}}, nil
}

func newTestChatFileResolver(storageImpl storage.Storage) *chatFileContentResolver {
	return &chatFileContentResolver{
		parserManager: &fakeChatFileParserManager{parser: fakeChatFileParser{}},
		storage:       storageImpl,
		config: chatFileParseConfig{
			enabled:           true,
			maxFileSize:       1024,
			maxFiles:          2,
			maxConcurrent:     1,
			timeout:           time.Second,
			maxContentChars:   1000,
			maxTotalChars:     2000,
			allowedExtensions: map[documentparser.FileExtension]struct{}{documentparser.FileExtensionTXT: {}},
		},
		semaphore: make(chan struct{}, 1),
	}
}

func TestChatFileContentResolverResolve(t *testing.T) {
	fileStorage := &fakeChatFileStorage{body: []byte("file")}
	resolver := newTestChatFileResolver(fileStorage)
	input := &schema.Message{
		MultiContent: []schema.ChatMessagePart{
			{Type: schema.ChatMessagePartTypeText, Text: "summarize"},
			{Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URI: "uploads/report.txt", Name: "report.txt"}},
		},
	}

	err := resolver.Resolve(context.Background(), input, nil)

	require.NoError(t, err)
	require.Equal(t, "uploads/report.txt", fileStorage.key)
	require.Len(t, input.MultiContent, 2)
	require.Equal(t, "summarize", input.MultiContent[0].Text)
	require.Equal(t, schema.ChatMessagePartTypeText, input.MultiContent[1].Type)
	require.Contains(t, input.MultiContent[1].Text, "parsed document content")
}

func TestChatFileContentResolverLeavesNonFileInputUntouched(t *testing.T) {
	resolver := newTestChatFileResolver(&fakeChatFileStorage{})
	input := &schema.Message{
		Content: "hello",
		MultiContent: []schema.ChatMessagePart{
			{Type: schema.ChatMessagePartTypeText, Text: "hello"},
		},
	}

	err := resolver.Resolve(context.Background(), input, nil)

	require.NoError(t, err)
	require.Equal(t, "hello", input.Content)
	require.Equal(t, schema.ChatMessagePartTypeText, input.MultiContent[0].Type)
	require.Equal(t, "hello", input.MultiContent[0].Text)
}

func TestChatFileContentResolverSkipsAssistantArtifactFiles(t *testing.T) {
	fileStorage := &fakeChatFileStorage{body: []byte("file")}
	resolver := newTestChatFileResolver(fileStorage)
	input := &schema.Message{Content: "summarize"}
	history := []*schema.Message{{
		Role:    schema.Assistant,
		Content: "[report.docx](file://skill-artifacts/conversation/report.docx)",
		MultiContent: []schema.ChatMessagePart{{
			Type:    schema.ChatMessagePartTypeFileURL,
			FileURL: &schema.ChatMessageFileURL{URI: "artifacts/report.docx", Name: "report.docx"},
		}},
	}, {
		Role:    schema.Tool,
		Content: "uploaded file: file_key=skill-artifacts/conversation/report.docx, file_name=report.docx",
	}}

	require.NoError(t, resolver.Resolve(context.Background(), input, history))
	require.Empty(t, fileStorage.key)
	require.Len(t, history[0].MultiContent, 1)
	require.Equal(t, schema.ChatMessagePartTypeText, history[0].MultiContent[0].Type)
	require.Contains(t, history[0].MultiContent[0].Text, "report.docx")
	require.Equal(t, "report.docx", history[0].Content)
	require.NotContains(t, history[0].Content, "file://")
	require.NotContains(t, history[1].Content, "file_key=")
}

func TestChatFileContentResolverDoesNotParseHistoryFiles(t *testing.T) {
	fileStorage := &fakeChatFileStorage{body: []byte("file")}
	resolver := newTestChatFileResolver(fileStorage)
	input := &schema.Message{
		MultiContent: []schema.ChatMessagePart{{
			Type:    schema.ChatMessagePartTypeFileURL,
			FileURL: &schema.ChatMessageFileURL{URI: "uploads/report.txt", Name: "report.txt"},
		}},
	}
	history := []*schema.Message{{
		Role: schema.User,
		MultiContent: []schema.ChatMessagePart{{
			Type:    schema.ChatMessagePartTypeFileURL,
			FileURL: &schema.ChatMessageFileURL{URI: "uploads/report.txt", Name: "report.txt"},
		}},
	}}

	require.NoError(t, resolver.Resolve(context.Background(), input, history))
	require.Contains(t, input.MultiContent[0].Text, "parsed document content")
	require.Equal(t, "uploads/report.txt", fileStorage.key)
	require.Len(t, history[0].MultiContent, 1)
	require.Equal(t, schema.ChatMessagePartTypeText, history[0].MultiContent[0].Type)
	require.Contains(t, history[0].MultiContent[0].Text, "历史附件")
	require.Contains(t, history[0].MultiContent[0].Text, "report.txt")
	require.Contains(t, history[0].MultiContent[0].Text, "uploads/report.txt")
}

func TestChatFileContentResolverHistoryOnlyDoesNotConsumeParseLimit(t *testing.T) {
	fileStorage := &fakeChatFileStorage{body: []byte("file")}
	resolver := newTestChatFileResolver(fileStorage)
	resolver.config.maxFiles = 1
	history := []*schema.Message{{
		Role: schema.User,
		MultiContent: []schema.ChatMessagePart{
			{Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URI: "uploads/old-a.txt", Name: "old-a.txt"}},
			{Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URI: "uploads/old-b.txt", Name: "old-b.txt"}},
		}},
	}

	require.NoError(t, resolver.Resolve(context.Background(), &schema.Message{Content: "run skill"}, history))
	require.Empty(t, fileStorage.key)
	require.Contains(t, history[0].MultiContent[0].Text, "old-a.txt")
	require.Contains(t, history[0].MultiContent[1].Text, "old-b.txt")
}

func TestSelectLatestFileMatchPrefersNewestDuplicate(t *testing.T) {
	files := []conversationSearchFile{
		{URI: "uploads/old/report.xlsx", Name: "report.xlsx"},
		{URI: "uploads/new/report.xlsx", Name: "report.xlsx"},
	}

	selected := selectLatestFileMatch(files, "report.xlsx")
	require.Len(t, selected, 1)
	require.Equal(t, "uploads/new/report.xlsx", selected[0].URI)

	selected = selectLatestFileMatch(files, "uploads/old/report.xlsx")
	require.Len(t, selected, 1)
	require.Equal(t, "uploads/old/report.xlsx", selected[0].URI)
}
