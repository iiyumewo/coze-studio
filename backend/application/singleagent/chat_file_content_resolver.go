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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	crossmessage "github.com/coze-dev/coze-studio/backend/crossdomain/message"
	messagemodel "github.com/coze-dev/coze-studio/backend/crossdomain/message/model"
	singleagent "github.com/coze-dev/coze-studio/backend/domain/agent/singleagent/service"
	"github.com/coze-dev/coze-studio/backend/infra/cache"
	documentparser "github.com/coze-dev/coze-studio/backend/infra/document/parser"
	"github.com/coze-dev/coze-studio/backend/infra/storage"
	"github.com/coze-dev/coze-studio/backend/pkg/envkey"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
)

const (
	chatFileParseEnabledEnv           = "COZE_CHAT_FILE_PARSE_ENABLED"
	chatFileParseMaxFileSizeEnv       = "COZE_CHAT_FILE_PARSE_MAX_FILE_SIZE_BYTES"
	chatFileParseMaxFilesEnv          = "COZE_CHAT_FILE_PARSE_MAX_FILES_PER_RUN"
	chatFileParseTimeoutEnv           = "COZE_CHAT_FILE_PARSE_TIMEOUT_SECONDS"
	chatFileParseMaxConcurrentEnv     = "COZE_CHAT_FILE_PARSE_MAX_CONCURRENT"
	chatFileParseMaxContentCharsEnv   = "COZE_CHAT_FILE_PARSE_MAX_CONTENT_CHARS"
	chatFileParseMaxTotalCharsEnv     = "COZE_CHAT_FILE_PARSE_MAX_TOTAL_CONTENT_CHARS"
	chatFileParseAllowedExtensionsEnv = "COZE_CHAT_FILE_PARSE_ALLOWED_EXTENSIONS"
	chatFileParseExtractTablesEnv     = "COZE_CHAT_FILE_PARSE_EXTRACT_TABLES"
	chatFileParseExtractImagesEnv     = "COZE_CHAT_FILE_PARSE_EXTRACT_IMAGES"
	chatFileParseImageOCREnv          = "COZE_CHAT_FILE_PARSE_IMAGE_OCR"
	chatFileParseCacheEnabledEnv      = "COZE_CHAT_FILE_PARSE_CACHE_ENABLED"
	chatFileParseCacheTTLEnv          = "COZE_CHAT_FILE_PARSE_CACHE_TTL_SECONDS"
	chatFileToolEnabledEnv            = "COZE_CHAT_FILE_TOOL_ENABLED"

	defaultChatFileParseMaxFileSize     = 10 * 1024 * 1024
	defaultChatFileParseMaxFiles        = 3
	defaultChatFileParseTimeout         = 30 * time.Second
	defaultChatFileParseMaxConcurrent   = 2
	defaultChatFileParseMaxContentChars = 80000
	defaultChatFileParseMaxTotalChars   = 120000
	defaultChatFileParseChunkSize       = 8000
	defaultChatFileParseCacheTTL        = time.Hour
)

type chatFileParseConfig struct {
	enabled           bool
	maxFileSize       int
	maxFiles          int
	maxConcurrent     int
	timeout           time.Duration
	maxContentChars   int
	maxTotalChars     int
	extractTables     bool
	extractImages     bool
	imageOCR          bool
	cacheEnabled      bool
	cacheTTL          time.Duration
	toolEnabled       bool
	allowedExtensions map[documentparser.FileExtension]struct{}
}

type chatFileContentResolver struct {
	parserManager documentparser.Manager
	storage       storage.Storage
	cache         cache.Cmdable
	config        chatFileParseConfig
	semaphore     chan struct{}
}

type conversationSearchFile struct {
	URI  string
	Name string
}

func newChatFileContentResolver(parserManager documentparser.Manager, objectStorage storage.Storage, cacheClient cache.Cmdable) singleagent.FileContentResolver {
	config := loadChatFileParseConfig()
	logs.Infof("[ChatFileContentResolver] init enabled=%t parser_manager_present=%t storage_present=%t", config.enabled, parserManager != nil, objectStorage != nil)
	if !config.enabled || parserManager == nil || objectStorage == nil {
		return nil
	}

	return &chatFileContentResolver{
		parserManager: parserManager,
		storage:       objectStorage,
		cache:         cacheClient,
		config:        config,
		semaphore:     make(chan struct{}, config.maxConcurrent),
	}
}

func loadChatFileParseConfig() chatFileParseConfig {
	maxConcurrent := envkey.GetIntD(chatFileParseMaxConcurrentEnv, defaultChatFileParseMaxConcurrent)
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}

	maxFiles := envkey.GetIntD(chatFileParseMaxFilesEnv, defaultChatFileParseMaxFiles)
	if maxFiles < 1 {
		maxFiles = 1
	}

	maxContentChars := envkey.GetIntD(chatFileParseMaxContentCharsEnv, defaultChatFileParseMaxContentChars)
	if maxContentChars < 1 {
		maxContentChars = defaultChatFileParseMaxContentChars
	}

	maxTotalChars := envkey.GetIntD(chatFileParseMaxTotalCharsEnv, defaultChatFileParseMaxTotalChars)
	if maxTotalChars < maxContentChars {
		maxTotalChars = maxContentChars
	}

	maxFileSize := envkey.GetIntD(chatFileParseMaxFileSizeEnv, defaultChatFileParseMaxFileSize)
	if maxFileSize < 1 {
		maxFileSize = defaultChatFileParseMaxFileSize
	}

	timeoutSeconds := envkey.GetIntD(chatFileParseTimeoutEnv, int(defaultChatFileParseTimeout/time.Second))
	if timeoutSeconds < 1 {
		timeoutSeconds = int(defaultChatFileParseTimeout / time.Second)
	}
	cacheTTLSeconds := envkey.GetIntD(chatFileParseCacheTTLEnv, int(defaultChatFileParseCacheTTL/time.Second))
	if cacheTTLSeconds < 1 {
		cacheTTLSeconds = int(defaultChatFileParseCacheTTL / time.Second)
	}

	return chatFileParseConfig{
		enabled:           envkey.GetBoolD(chatFileParseEnabledEnv, false),
		maxFileSize:       maxFileSize,
		maxFiles:          maxFiles,
		maxConcurrent:     maxConcurrent,
		timeout:           time.Duration(timeoutSeconds) * time.Second,
		maxContentChars:   maxContentChars,
		maxTotalChars:     maxTotalChars,
		extractTables:     envkey.GetBoolD(chatFileParseExtractTablesEnv, true),
		extractImages:     envkey.GetBoolD(chatFileParseExtractImagesEnv, false),
		imageOCR:          envkey.GetBoolD(chatFileParseImageOCREnv, false),
		cacheEnabled:      envkey.GetBoolD(chatFileParseCacheEnabledEnv, true),
		cacheTTL:          time.Duration(cacheTTLSeconds) * time.Second,
		toolEnabled:       envkey.GetBoolD(chatFileToolEnabledEnv, true),
		allowedExtensions: parseAllowedExtensions(envkey.GetStringD(chatFileParseAllowedExtensionsEnv, "docx,pdf,txt,md,csv,xlsx")),
	}
}

func parseAllowedExtensions(raw string) map[documentparser.FileExtension]struct{} {
	allowed := make(map[documentparser.FileExtension]struct{})
	for _, item := range strings.Split(raw, ",") {
		extension := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(item)), ".")
		if extension == "" {
			continue
		}
		allowed[documentparser.FileExtension(extension)] = struct{}{}
	}
	return allowed
}

func (r *chatFileContentResolver) Resolve(ctx context.Context, input *schema.Message, history []*schema.Message) error {
	logs.CtxInfof(ctx, "[ChatFileContentResolver] resolve start input_parts=%d history_messages=%d", messagePartCount(input), len(history))
	if input == nil && len(history) == 0 {
		return nil
	}

	messageCount := 0
	contentChars := 0
	resolvedContents := make(map[string]string)
	if input != nil {
		var err error
		messageCount, contentChars, err = r.resolveCurrentMessage(ctx, input, resolvedContents, messageCount, contentChars)
		if err != nil {
			return err
		}
	}

	for _, message := range history {
		// Assistant file messages are delivery artifacts. Keep a text summary in
		// model history, but do not replay file_url parts into the LLM pipeline:
		// the generic single-agent history path does not support those parts.
		if message != nil && message.Role == schema.Assistant {
			message.Content = sanitizeInternalFileReferences(message.Content)
			replaceAssistantFileParts(message)
			continue
		}
		if message != nil && message.Role == schema.Tool {
			message.Content = sanitizeInternalFileReferences(message.Content)
			continue
		}
		replaceHistoricalFileParts(message)
	}

	return nil
}

func (r *chatFileContentResolver) resolveCurrentMessage(
	ctx context.Context,
	message *schema.Message,
	resolvedContents map[string]string,
	messageCount int,
	contentChars int,
) (int, int, error) {
	if message == nil || len(message.MultiContent) == 0 {
		return messageCount, contentChars, nil
	}

	resolved := make([]schema.ChatMessagePart, 0, len(message.MultiContent))
	for _, part := range message.MultiContent {
		if part.Type != schema.ChatMessagePartTypeFileURL || part.FileURL == nil || part.FileURL.URI == "" {
			resolved = append(resolved, part)
			continue
		}
		logs.CtxInfof(ctx, "[ChatFileContentResolver] current file part detected, name=%q, uri_present=%t", part.FileURL.Name, part.FileURL.URI != "")

		if cachedContent, ok := resolvedContents[part.FileURL.URI]; ok {
			resolved = append(resolved, schema.ChatMessagePart{Type: schema.ChatMessagePartTypeText, Text: cachedContent})
			continue
		}
		messageCount++
		if messageCount > r.config.maxFiles {
			return messageCount, contentChars, fmt.Errorf("chat file parse limit exceeded: max files per run is %d", r.config.maxFiles)
		}

		fileContent, err := r.resolveFile(ctx, part.FileURL)
		if err != nil {
			return messageCount, contentChars, err
		}

		remaining := r.config.maxTotalChars - contentChars
		if remaining <= 0 {
			return messageCount, contentChars, fmt.Errorf("chat file parse limit exceeded: max total content is %d characters", r.config.maxTotalChars)
		}
		fileContent = truncateRunes(fileContent, minInt(remaining, r.config.maxContentChars))
		contentChars += len([]rune(fileContent))
		resolvedContents[part.FileURL.URI] = fileContent
		resolved = append(resolved, schema.ChatMessagePart{
			Type: schema.ChatMessagePartTypeText,
			Text: fileContent,
		})
	}

	message.MultiContent = resolved
	return messageCount, contentChars, nil
}

// sanitizeInternalFileReferences removes storage/workspace references from
// legacy history. New file_upload responses never expose these values, but
// existing conversations may contain model-generated file:// links or the old
// tool response with file_key. Neither is a usable model-facing download API.
func sanitizeInternalFileReferences(content string) string {
	for {
		start := strings.Index(content, "file://skill-artifacts/")
		if start < 0 {
			break
		}
		end := start
		for end < len(content) {
			switch content[end] {
			case ')', ']', '}', '>', '"', '\'', ' ', '\n', '\r', '\t':
				goto foundEnd
			default:
				end++
			}
		}
	foundEnd:
		linkStart := strings.LastIndex(content[:start], "](")
		if linkStart >= 0 {
			labelStart := strings.LastIndex(content[:linkStart], "[")
			tailStart := end
			if tailStart < len(content) && content[tailStart] == ')' {
				tailStart++
			}
			if labelStart >= 0 {
				content = content[:labelStart] + content[labelStart+1:linkStart] + content[tailStart:]
			} else {
				content = content[:linkStart] + content[tailStart:]
			}
		} else {
			content = content[:start] + content[end:]
		}
	}

	for {
		keyStart := strings.Index(content, "file_key=")
		if keyStart < 0 {
			break
		}
		keyEnd := keyStart + len("file_key=")
		for keyEnd < len(content) && content[keyEnd] != ',' && content[keyEnd] != ' ' && content[keyEnd] != '\n' {
			keyEnd++
		}
		content = content[:keyStart] + content[keyEnd:]
	}
	return content
}

func replaceAssistantFileParts(message *schema.Message) {
	if message == nil || len(message.MultiContent) == 0 {
		return
	}
	parts := make([]schema.ChatMessagePart, 0, len(message.MultiContent))
	for _, part := range message.MultiContent {
		if part.Type != schema.ChatMessagePartTypeFileURL || part.FileURL == nil {
			parts = append(parts, part)
			continue
		}
		name := part.FileURL.Name
		if name == "" {
			name = path.Base(strings.Split(part.FileURL.URI, "?")[0])
		}
		if name != "" && name != "." {
			parts = append(parts, schema.ChatMessagePart{
				Type: schema.ChatMessagePartTypeText,
				Text: "[已交付文件: " + name + "]",
			})
		}
	}
	message.MultiContent = parts
}

func replaceHistoricalFileParts(message *schema.Message) {
	if message == nil || len(message.MultiContent) == 0 {
		return
	}
	parts := make([]schema.ChatMessagePart, 0, len(message.MultiContent))
	for _, part := range message.MultiContent {
		if part.Type != schema.ChatMessagePartTypeFileURL || part.FileURL == nil || part.FileURL.URI == "" {
			parts = append(parts, part)
			continue
		}
		name := part.FileURL.Name
		if name == "" {
			name = path.Base(strings.Split(part.FileURL.URI, "?")[0])
		}
		metadata := "[历史附件"
		if name != "" && name != "." {
			metadata += ": " + name
		}
		metadata += "; file_key: " + part.FileURL.URI + "]"
		parts = append(parts, schema.ChatMessagePart{
			Type: schema.ChatMessagePartTypeText,
			Text: metadata,
		})
	}
	message.MultiContent = parts
}

func messagePartCount(message *schema.Message) int {
	if message == nil {
		return 0
	}
	return len(message.MultiContent)
}

func (r *chatFileContentResolver) resolveFile(ctx context.Context, file *schema.ChatMessageFileURL) (string, error) {
	extension := fileExtension(file)
	if extension == "" {
		return "", fmt.Errorf("cannot determine uploaded file extension")
	}
	if _, ok := r.config.allowedExtensions[documentparser.FileExtension(extension)]; !ok {
		return "", fmt.Errorf("chat file type is not supported: %s", extension)
	}

	parseCtx, cancel := context.WithTimeout(ctx, r.config.timeout)
	defer cancel()

	select {
	case r.semaphore <- struct{}{}:
		defer func() { <-r.semaphore }()
	case <-parseCtx.Done():
		return "", fmt.Errorf("chat file parse queue timeout: %w", parseCtx.Err())
	}

	cacheKey := r.parseCacheKey(parseCtx, file.URI)
	if r.cache != nil && r.config.cacheEnabled {
		if cached, cacheErr := r.cache.Get(parseCtx, cacheKey).Result(); cacheErr == nil && cached != "" {
			return cached, nil
		}
	}

	body, err := r.storage.GetObject(parseCtx, file.URI)
	if err != nil {
		return "", fmt.Errorf("read uploaded file failed: %w", err)
	}
	if len(body) > r.config.maxFileSize {
		return "", fmt.Errorf("uploaded file is too large: %d bytes", len(body))
	}

	fileParser, err := r.parserManager.GetParser(&documentparser.Config{
		FileExtension: documentparser.FileExtension(extension),
		ParsingStrategy: &documentparser.ParsingStrategy{
			ExtractImage:  r.config.extractImages,
			ExtractTable:  r.config.extractTables,
			ImageOCR:      r.config.imageOCR,
			DataStartLine: 1,
		},
		ChunkingStrategy: &documentparser.ChunkingStrategy{
			ChunkType: documentparser.ChunkTypeDefault,
			ChunkSize: defaultChatFileParseChunkSize,
			Separator: "\n\n",
			TrimSpace: true,
		},
	})
	if err != nil {
		return "", fmt.Errorf("create parser for %s failed: %w", extension, err)
	}

	docs, err := fileParser.Parse(parseCtx, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("parse uploaded %s failed: %w", extension, err)
	}
	logs.CtxInfof(ctx, "[ChatFileContentResolver] parsed file, extension=%s, documents=%d", extension, len(docs))

	var content strings.Builder
	fileName := file.Name
	if fileName == "" {
		fileName = path.Base(strings.Split(file.URI, "?")[0])
	}
	if fileName != "" && fileName != "." {
		fmt.Fprintf(&content, "[附件: %s]\n", fileName)
	}
	for _, doc := range docs {
		if doc == nil || strings.TrimSpace(doc.Content) == "" {
			continue
		}
		if content.Len() > 0 {
			content.WriteString("\n\n")
		}
		content.WriteString(doc.Content)
	}
	if strings.TrimSpace(content.String()) == "" {
		return "", fmt.Errorf("uploaded %s contains no readable content", extension)
	}

	parsedContent := content.String()
	if r.cache != nil && r.config.cacheEnabled {
		if err := r.cache.Set(parseCtx, cacheKey, parsedContent, r.config.cacheTTL).Err(); err != nil {
			logs.CtxWarnf(ctx, "[ChatFileContentResolver] cache parsed file failed, extension=%s, err=%v", extension, err)
		}
	}

	return parsedContent, nil
}

func (r *chatFileContentResolver) SearchConversation(
	ctx context.Context,
	conversationID int64,
	agentID int64,
	userID string,
	fileRef string,
	query string,
) (string, error) {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("file search user id is invalid")
	}

	messageList, err := crossmessage.DefaultSVC().MessageList(ctx, &crossmessage.MessageListRequest{
		ConversationID: conversationID,
		Limit:          100,
		UserID:         uid,
		BizID:          agentID,
		OrderBy:        ptr.Of("ASC"),
	})
	if err != nil {
		return "", fmt.Errorf("list conversation files failed: %w", err)
	}

	files := make([]conversationSearchFile, 0)
	seen := make(map[string]struct{})
	fileRefLower := strings.ToLower(strings.TrimSpace(fileRef))
	for _, message := range messageList.Messages {
		for _, content := range message.MultiContent {
			if content == nil || content.Type != messagemodel.InputTypeFile || content.Uri == nil || *content.Uri == "" {
				continue
			}
			uri := *content.Uri
			if _, ok := seen[uri]; ok {
				continue
			}
			name := ""
			if content.Name != nil {
				name = strings.TrimSpace(*content.Name)
			}
			if name == "" {
				name = path.Base(strings.Split(uri, "?")[0])
			}
			if fileRefLower != "" && !strings.Contains(strings.ToLower(name), fileRefLower) && !strings.Contains(strings.ToLower(uri), fileRefLower) {
				continue
			}
			seen[uri] = struct{}{}
			files = append(files, conversationSearchFile{URI: uri, Name: name})
		}
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no matching files found in current conversation")
	}
	if strings.TrimSpace(fileRef) != "" {
		files = selectLatestFileMatch(files, fileRef)
	}

	var result strings.Builder
	for _, file := range files {
		content, parseErr := r.resolveFile(ctx, &schema.ChatMessageFileURL{URI: file.URI, Name: file.Name})
		if parseErr != nil {
			return "", parseErr
		}
		if result.Len() > 0 {
			result.WriteString("\n\n")
		}
		result.WriteString(content)
	}

	if query != "" {
		logs.CtxInfof(ctx, "[ChatFileContentResolver] file search completed, files=%d, query_chars=%d", len(files), len([]rune(query)))
	}
	return truncateRunes(result.String(), r.config.maxTotalChars), nil
}

func selectLatestFileMatch(files []conversationSearchFile, fileRef string) []conversationSearchFile {
	// MessageList is requested in ascending order, so the last matching
	// attachment is the newest one when duplicate filenames are uploaded.
	needle := strings.ToLower(strings.TrimSpace(fileRef))
	for i := len(files) - 1; i >= 0; i-- {
		if files[i].URI == strings.TrimSpace(fileRef) || strings.EqualFold(files[i].Name, strings.TrimSpace(fileRef)) || strings.Contains(strings.ToLower(files[i].Name), needle) || strings.Contains(strings.ToLower(files[i].URI), needle) {
			return files[i : i+1]
		}
	}
	return files
}

func (r *chatFileContentResolver) parseCacheKey(ctx context.Context, uri string) string {
	version := ""
	if info, err := r.storage.HeadObject(ctx, uri); err == nil && info != nil {
		version = info.ETag
	}
	configVersion := fmt.Sprintf("tables=%t,images=%t,ocr=%t", r.config.extractTables, r.config.extractImages, r.config.imageOCR)
	sum := sha256.Sum256([]byte(uri + "|" + version + "|" + configVersion))
	return "chat:file-parse:" + hex.EncodeToString(sum[:])
}

func fileExtension(file *schema.ChatMessageFileURL) string {
	name := file.Name
	if name == "" {
		name = file.URI
	}
	name = strings.Split(name, "?")[0]
	return strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "\n[附件内容已达到长度上限，后续内容未注入模型]"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ singleagent.FileContentResolver = (*chatFileContentResolver)(nil)
