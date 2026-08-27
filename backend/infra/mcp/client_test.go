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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestClientTest(t *testing.T) {
	tests := []struct {
		name           string
		expectedHeader string
		authentication Authentication
	}{
		{
			name:           "without authentication",
			authentication: Authentication{Type: AuthenticationTypeNone},
		},
		{
			name:           "with custom header authentication",
			expectedHeader: "Bearer test-secret",
			authentication: Authentication{
				Type:        AuthenticationTypeHeader,
				HeaderName:  "Authorization",
				HeaderValue: "Bearer test-secret",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testServer, deleteRequests := newMCPTestServer(t, test.expectedHeader)
			defer testServer.Close()

			result, err := NewClient().Test(context.Background(), &TestRequest{
				ServerURL:      testServer.URL,
				Authentication: test.authentication,
			})
			if err != nil {
				t.Fatalf("Test() error = %v", err)
			}

			if result.ServerName != "test-server" || result.ServerTitle != "Test Server" {
				t.Fatalf("server info = %q/%q, want test-server/Test Server", result.ServerName, result.ServerTitle)
			}
			if result.ServerVersion != "1.2.3" {
				t.Fatalf("server version = %q, want 1.2.3", result.ServerVersion)
			}
			if result.ProtocolVersion == "" {
				t.Fatal("protocol version is empty")
			}
			if result.Instructions != "Use the test tools." {
				t.Fatalf("instructions = %q, want %q", result.Instructions, "Use the test tools.")
			}
			if len(result.Tools) != 2 {
				t.Fatalf("tool count = %d, want 2", len(result.Tools))
			}
			if result.Tools[0].Name != "lookup" || result.Tools[1].Name != "write" {
				t.Fatalf("tool names = %q/%q, want lookup/write", result.Tools[0].Name, result.Tools[1].Name)
			}
			if !json.Valid(result.Tools[0].InputSchema) || !json.Valid(result.Tools[0].OutputSchema) {
				t.Fatal("normalized tool schemas are not valid JSON")
			}
			if result.Tools[0].Annotations == nil || !result.Tools[0].Annotations.ReadOnlyHint {
				t.Fatal("tool annotations were not normalized")
			}
			if deleteRequests.Load() == 0 {
				t.Fatal("MCP session was not closed")
			}
		})
	}
}

func TestClientCallTool(t *testing.T) {
	testServer, deleteRequests := newMCPTestServer(t, "Bearer test-secret")
	defer testServer.Close()

	toolClient := NewToolClient()
	authentication := Authentication{
		Type:        AuthenticationTypeHeader,
		HeaderName:  "Authorization",
		HeaderValue: "Bearer test-secret",
	}

	result, err := toolClient.CallTool(context.Background(), &CallToolRequest{
		ServerURL:      testServer.URL,
		Authentication: authentication,
		ToolName:       "lookup",
		Arguments:      json.RawMessage(`{"key":"coze"}`),
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatal("CallTool() unexpectedly returned a tool error")
	}
	if result.Output != "found coze\n{\"value\":\"coze\"}" {
		t.Fatalf("CallTool() output = %q", result.Output)
	}

	errorResult, err := toolClient.CallTool(context.Background(), &CallToolRequest{
		ServerURL:      testServer.URL,
		Authentication: authentication,
		ToolName:       "write",
		Arguments:      json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("CallTool() tool error = %v, want MCP result", err)
	}
	if !errorResult.IsError || errorResult.Output != "write rejected" {
		t.Fatalf("CallTool() error result = %#v", errorResult)
	}
	if deleteRequests.Load() < 2 {
		t.Fatal("MCP tool-call sessions were not closed")
	}
}

func TestClientDoesNotAdvertiseRoots(t *testing.T) {
	testServer, _ := newMCPTestServer(t, "")
	defer testServer.Close()

	transport := &initializeCaptureRoundTripper{base: http.DefaultTransport}
	tester := newClient(clientOptions{transport: transport})
	_, err := tester.Test(context.Background(), &TestRequest{
		ServerURL:      testServer.URL,
		Authentication: Authentication{Type: AuthenticationTypeNone},
	})
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}

	capabilities := transport.initializeCapabilities()
	if capabilities == nil {
		t.Fatal("initialize request was not captured")
	}
	if _, exists := capabilities["roots"]; exists {
		t.Fatalf("initialize capabilities = %#v, want roots disabled", capabilities)
	}
}

func TestValidateRequest(t *testing.T) {
	tests := []struct {
		name    string
		request *TestRequest
	}{
		{name: "nil request"},
		{name: "empty URL", request: &TestRequest{Authentication: Authentication{Type: AuthenticationTypeNone}}},
		{name: "unsupported scheme", request: &TestRequest{ServerURL: "ftp://example.com/mcp", Authentication: Authentication{Type: AuthenticationTypeNone}}},
		{name: "missing host", request: &TestRequest{ServerURL: "http:///mcp", Authentication: Authentication{Type: AuthenticationTypeNone}}},
		{name: "credentials in URL", request: &TestRequest{ServerURL: "https://user:secret@example.com/mcp", Authentication: Authentication{Type: AuthenticationTypeNone}}},
		{name: "URL fragment", request: &TestRequest{ServerURL: "https://example.com/mcp#fragment", Authentication: Authentication{Type: AuthenticationTypeNone}}},
		{name: "unsupported authentication", request: &TestRequest{ServerURL: "https://example.com/mcp", Authentication: Authentication{Type: "oauth"}}},
		{name: "missing header name", request: &TestRequest{ServerURL: "https://example.com/mcp", Authentication: Authentication{Type: AuthenticationTypeHeader, HeaderValue: "secret"}}},
		{name: "missing header value", request: &TestRequest{ServerURL: "https://example.com/mcp", Authentication: Authentication{Type: AuthenticationTypeHeader, HeaderName: "Authorization"}}},
		{name: "invalid header name", request: &TestRequest{ServerURL: "https://example.com/mcp", Authentication: Authentication{Type: AuthenticationTypeHeader, HeaderName: "Bad Header", HeaderValue: "secret"}}},
		{name: "header injection", request: &TestRequest{ServerURL: "https://example.com/mcp", Authentication: Authentication{Type: AuthenticationTypeHeader, HeaderName: "Authorization", HeaderValue: "secret\r\nX-Injected: true"}}},
		{name: "reserved header", request: &TestRequest{ServerURL: "https://example.com/mcp", Authentication: Authentication{Type: AuthenticationTypeHeader, HeaderName: "Mcp-Session-Id", HeaderValue: "secret"}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := validateRequest(test.request)
			if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("validateRequest() error = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

type initializeCaptureRoundTripper struct {
	base         http.RoundTripper
	mu           sync.Mutex
	capabilities map[string]any
}

func (t *initializeCaptureRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Body != nil {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		request.Body = io.NopCloser(bytes.NewReader(body))

		var message struct {
			Method string `json:"method"`
			Params struct {
				Capabilities map[string]any `json:"capabilities"`
			} `json:"params"`
		}
		if json.Unmarshal(body, &message) == nil && message.Method == "initialize" {
			t.mu.Lock()
			t.capabilities = message.Params.Capabilities
			t.mu.Unlock()
		}
	}

	return t.base.RoundTrip(request)
}

func (t *initializeCaptureRoundTripper) initializeCapabilities() map[string]any {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.capabilities
}

func TestClientTestTimesOut(t *testing.T) {
	var once sync.Once
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		once.Do(func() { close(requestStarted) })
		select {
		case <-request.Context().Done():
		case <-time.After(250 * time.Millisecond):
		}
	}))
	defer server.Close()

	tester := newClient(clientOptions{timeout: 50 * time.Millisecond})
	startedAt := time.Now()
	_, err := tester.Test(context.Background(), &TestRequest{
		ServerURL:      server.URL,
		Authentication: Authentication{Type: AuthenticationTypeNone},
	})
	if !errors.Is(err, ErrConnectServer) {
		t.Fatalf("Test() error = %v, want ErrConnectServer", err)
	}
	if elapsed := time.Since(startedAt); elapsed >= 250*time.Millisecond {
		t.Fatalf("Test() took %s, want client timeout before server response", elapsed)
	}

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("MCP request was not sent")
	}
}

func TestClientTestRejectsRedirect(t *testing.T) {
	var targetRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		targetRequests.Add(1)
	}))
	defer target.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()

	_, err := NewClient().Test(context.Background(), &TestRequest{
		ServerURL: redirect.URL,
		Authentication: Authentication{
			Type:        AuthenticationTypeHeader,
			HeaderName:  "Authorization",
			HeaderValue: "Bearer must-not-leak",
		},
	})
	if !errors.Is(err, ErrConnectServer) || !strings.Contains(err.Error(), ErrRedirectNotAllowed.Error()) {
		t.Fatalf("Test() error = %v, want redirect rejection", err)
	}
	if targetRequests.Load() != 0 {
		t.Fatal("redirect target received a request")
	}
}

func TestClientTestLimitsResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(strings.Repeat("x", 1024)))
	}))
	defer server.Close()

	tester := newClient(clientOptions{maxResponseBodyBytes: 64})
	_, err := tester.Test(context.Background(), &TestRequest{
		ServerURL:      server.URL,
		Authentication: Authentication{Type: AuthenticationTypeNone},
	})
	if !errors.Is(err, ErrConnectServer) {
		t.Fatalf("Test() error = %v, want ErrConnectServer", err)
	}
}

func TestClientTestReportsToolDiscoveryFailure(t *testing.T) {
	sdkServer := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}, nil)
	streamableHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return sdkServer
	}, nil)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			response.WriteHeader(http.StatusInternalServerError)
			return
		}
		if bytes.Contains(body, []byte(`"method":"tools/list"`)) {
			response.WriteHeader(http.StatusInternalServerError)
			return
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		streamableHandler.ServeHTTP(response, request)
	}))
	defer server.Close()

	_, err := NewClient().Test(context.Background(), &TestRequest{
		ServerURL:      server.URL,
		Authentication: Authentication{Type: AuthenticationTypeNone},
	})
	if !errors.Is(err, ErrDiscoverTools) {
		t.Fatalf("Test() error = %v, want ErrDiscoverTools", err)
	}
}

func TestSafeStageErrorRedactsSecret(t *testing.T) {
	const secret = "Bearer private-value"
	err := safeStageError(ErrConnectServer, errors.New("server repeated "+secret), secret)
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("safeStageError() leaked secret: %v", err)
	}
	if !errors.Is(err, ErrConnectServer) {
		t.Fatalf("safeStageError() error = %v, want ErrConnectServer", err)
	}
}

func newMCPTestServer(t *testing.T, expectedAuthorization string) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	destructiveHint := false
	sdkServer := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "test-server",
		Title:   "Test Server",
		Version: "1.2.3",
	}, &mcpsdk.ServerOptions{
		Instructions: "Use the test tools.",
		PageSize:     1,
	})
	sdkServer.AddTool(&mcpsdk.Tool{
		Name:         "lookup",
		Title:        "Lookup",
		Description:  "Looks up a value.",
		InputSchema:  map[string]any{"type": "object", "properties": map[string]any{"key": map[string]any{"type": "string"}}},
		OutputSchema: map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}}},
		Annotations: &mcpsdk.ToolAnnotations{
			Title:           "Lookup",
			ReadOnlyHint:    true,
			DestructiveHint: &destructiveHint,
		},
	}, func(_ context.Context, request *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		var arguments struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(request.Params.Arguments, &arguments); err != nil {
			return nil, err
		}
		return &mcpsdk.CallToolResult{
			Content:           []mcpsdk.Content{&mcpsdk.TextContent{Text: "found " + arguments.Key}},
			StructuredContent: map[string]any{"value": arguments.Key},
		}, nil
	})
	sdkServer.AddTool(&mcpsdk.Tool{
		Name:        "write",
		Description: "Writes a value.",
		InputSchema: map[string]any{"type": "object"},
	}, func(context.Context, *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "write rejected"}},
			IsError: true,
		}, nil
	})

	streamableHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return sdkServer
	}, nil)
	var deleteRequests atomic.Int32
	testServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != expectedAuthorization {
			http.Error(response, "unauthorized", http.StatusUnauthorized)
			return
		}
		if request.Method == http.MethodDelete {
			deleteRequests.Add(1)
		}
		streamableHandler.ServeHTTP(response, request)
	}))

	return testServer, &deleteRequests
}
