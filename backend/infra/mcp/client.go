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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/net/http/httpguts"
)

const (
	defaultTimeout               = 15 * time.Second
	defaultMaxResponseBodyBytes  = int64(8 << 20)
	maxServerURLLength           = 2048
	maxAuthenticationHeaderName  = 128
	maxAuthenticationHeaderValue = 2048
	clientName                   = "coze-studio-mcp-validator"
	clientVersion                = "0.1.0"
)

type clientOptions struct {
	timeout              time.Duration
	maxResponseBodyBytes int64
	transport            http.RoundTripper
}

type client struct {
	timeout              time.Duration
	maxResponseBodyBytes int64
	transport            http.RoundTripper
}

func NewClient() Client {
	return newClient(clientOptions{})
}

func newClient(options clientOptions) *client {
	if options.timeout <= 0 {
		options.timeout = defaultTimeout
	}
	if options.maxResponseBodyBytes <= 0 {
		options.maxResponseBodyBytes = defaultMaxResponseBodyBytes
	}
	if options.transport == nil {
		options.transport = http.DefaultTransport
	}

	return &client{
		timeout:              options.timeout,
		maxResponseBodyBytes: options.maxResponseBodyBytes,
		transport:            options.transport,
	}
}

func (c *client) Test(ctx context.Context, request *TestRequest) (*TestResult, error) {
	endpoint, authentication, err := validateRequest(request)
	if err != nil {
		return nil, err
	}

	testCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	httpClient := &http.Client{
		Timeout: c.timeout,
		Transport: &requestRoundTripper{
			base:                 c.transport,
			authentication:       authentication,
			maxResponseBodyBytes: c.maxResponseBodyBytes,
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return ErrRedirectNotAllowed
		},
	}

	sdkClient := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    clientName,
		Version: clientVersion,
	}, nil)
	session, err := sdkClient.Connect(testCtx, &mcpsdk.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           httpClient,
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		return nil, safeStageError(ErrConnectServer, err, authentication.HeaderValue)
	}
	defer session.Close()

	initializeResult := session.InitializeResult()
	if initializeResult == nil || initializeResult.ServerInfo == nil {
		return nil, fmt.Errorf("%w: server implementation information is missing", ErrConnectServer)
	}

	tools := make([]*Tool, 0)
	for sdkTool, listErr := range session.Tools(testCtx, nil) {
		if listErr != nil {
			return nil, safeStageError(ErrDiscoverTools, listErr, authentication.HeaderValue)
		}

		tool, normalizeErr := normalizeTool(sdkTool)
		if normalizeErr != nil {
			return nil, safeStageError(ErrNormalizeTool, normalizeErr, authentication.HeaderValue)
		}
		tools = append(tools, tool)
	}

	return &TestResult{
		ServerName:      initializeResult.ServerInfo.Name,
		ServerTitle:     initializeResult.ServerInfo.Title,
		ServerVersion:   initializeResult.ServerInfo.Version,
		ProtocolVersion: initializeResult.ProtocolVersion,
		Instructions:    initializeResult.Instructions,
		Tools:           tools,
	}, nil
}

func validateRequest(request *TestRequest) (string, Authentication, error) {
	if request == nil {
		return "", Authentication{}, fmt.Errorf("%w: request is required", ErrInvalidRequest)
	}

	endpoint := strings.TrimSpace(request.ServerURL)
	if endpoint == "" {
		return "", Authentication{}, fmt.Errorf("%w: server URL is required", ErrInvalidRequest)
	}
	if len(endpoint) > maxServerURLLength {
		return "", Authentication{}, fmt.Errorf("%w: server URL is too long", ErrInvalidRequest)
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil || parsedURL.Host == "" {
		return "", Authentication{}, fmt.Errorf("%w: server URL is invalid", ErrInvalidRequest)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", Authentication{}, fmt.Errorf("%w: server URL must use HTTP or HTTPS", ErrInvalidRequest)
	}
	if parsedURL.User != nil {
		return "", Authentication{}, fmt.Errorf("%w: credentials in the server URL are not allowed", ErrInvalidRequest)
	}
	if parsedURL.Fragment != "" {
		return "", Authentication{}, fmt.Errorf("%w: server URL must not contain a fragment", ErrInvalidRequest)
	}

	authentication := request.Authentication
	switch authentication.Type {
	case AuthenticationTypeNone:
		return endpoint, Authentication{Type: AuthenticationTypeNone}, nil
	case AuthenticationTypeHeader:
		return validateHeaderAuthentication(endpoint, authentication)
	default:
		return "", Authentication{}, fmt.Errorf("%w: unsupported authentication type", ErrInvalidRequest)
	}
}

func validateHeaderAuthentication(endpoint string, authentication Authentication) (string, Authentication, error) {
	headerName := strings.TrimSpace(authentication.HeaderName)
	if headerName == "" {
		return "", Authentication{}, fmt.Errorf("%w: authentication header name is required", ErrInvalidRequest)
	}
	if len(headerName) > maxAuthenticationHeaderName || !httpguts.ValidHeaderFieldName(headerName) {
		return "", Authentication{}, fmt.Errorf("%w: authentication header name is invalid", ErrInvalidRequest)
	}
	if isReservedHeader(headerName) {
		return "", Authentication{}, fmt.Errorf("%w: authentication header name is reserved by HTTP or MCP", ErrInvalidRequest)
	}
	if strings.TrimSpace(authentication.HeaderValue) == "" {
		return "", Authentication{}, fmt.Errorf("%w: authentication header value is required", ErrInvalidRequest)
	}
	if len(authentication.HeaderValue) > maxAuthenticationHeaderValue || !httpguts.ValidHeaderFieldValue(authentication.HeaderValue) {
		return "", Authentication{}, fmt.Errorf("%w: authentication header value is invalid", ErrInvalidRequest)
	}

	return endpoint, Authentication{
		Type:        AuthenticationTypeHeader,
		HeaderName:  http.CanonicalHeaderKey(headerName),
		HeaderValue: authentication.HeaderValue,
	}, nil
}

func isReservedHeader(headerName string) bool {
	canonicalName := http.CanonicalHeaderKey(headerName)
	if strings.HasPrefix(canonicalName, "Mcp-") {
		return true
	}

	switch canonicalName {
	case "Accept", "Connection", "Content-Length", "Content-Type", "Host", "Last-Event-Id", "Transfer-Encoding":
		return true
	default:
		return false
	}
}

func normalizeTool(tool *mcpsdk.Tool) (*Tool, error) {
	if tool == nil {
		return nil, fmt.Errorf("tool is nil")
	}

	inputSchema, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("marshal input schema for tool %q: %w", tool.Name, err)
	}

	var outputSchema json.RawMessage
	if tool.OutputSchema != nil {
		outputSchema, err = json.Marshal(tool.OutputSchema)
		if err != nil {
			return nil, fmt.Errorf("marshal output schema for tool %q: %w", tool.Name, err)
		}
	}

	var annotations *ToolAnnotations
	if tool.Annotations != nil {
		annotations = &ToolAnnotations{
			Title:           tool.Annotations.Title,
			ReadOnlyHint:    tool.Annotations.ReadOnlyHint,
			DestructiveHint: tool.Annotations.DestructiveHint,
			IdempotentHint:  tool.Annotations.IdempotentHint,
			OpenWorldHint:   tool.Annotations.OpenWorldHint,
		}
	}

	return &Tool{
		Name:         tool.Name,
		Title:        tool.Title,
		Description:  tool.Description,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Annotations:  annotations,
	}, nil
}

func safeStageError(stage error, cause error, secret string) error {
	detail := cause.Error()
	if secret != "" {
		detail = strings.ReplaceAll(detail, secret, "[REDACTED]")
	}
	return fmt.Errorf("%w: %s", stage, detail)
}

type requestRoundTripper struct {
	base                 http.RoundTripper
	authentication       Authentication
	maxResponseBodyBytes int64
}

func (t *requestRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	clonedRequest := request.Clone(request.Context())
	clonedRequest.Header = request.Header.Clone()
	if t.authentication.Type == AuthenticationTypeHeader {
		clonedRequest.Header.Set(t.authentication.HeaderName, t.authentication.HeaderValue)
	}

	response, err := t.base.RoundTrip(clonedRequest)
	if err != nil {
		return nil, err
	}
	if response.Body != nil {
		response.Body = http.MaxBytesReader(nil, response.Body, t.maxResponseBodyBytes)
	}

	return response, nil
}
