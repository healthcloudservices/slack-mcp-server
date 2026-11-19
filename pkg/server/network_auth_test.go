package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNetworkAuthentication(t *testing.T) {
	for _, transport := range []string{"http", "sse"} {
		t.Run(transport, func(t *testing.T) {
			for _, tc := range []struct {
				name, key, header string
				want              int
			}{
				{"unconfigured key fails closed", "", "", http.StatusServiceUnavailable},
				{"missing bearer", "local-test-key", "", http.StatusUnauthorized},
				{"wrong bearer", "local-test-key", "Bearer wrong", http.StatusUnauthorized},
				{"raw key is not bearer authentication", "local-test-key", "local-test-key", http.StatusUnauthorized},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Setenv("SLACK_MCP_API_KEY", tc.key)
					t.Setenv("SLACK_MCP_SSE_API_KEY", "")
					s := &MCPServer{server: server.NewMCPServer("auth-test", "1.0.0"), logger: zap.NewNop()}
					var h http.Handler
					if transport == "http" {
						h = s.ServeHTTP("").Handler
					} else {
						h = s.ServeSSE("").Handler
					}
					for _, path := range []string{"/mcp", "/sse", "/message"} {
						for _, method := range []string{"initialize", "tools/list", "resources/read"} {
							body := `{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":{}}`
							req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
							req.Header.Set("Content-Type", "application/json")
							req.Header.Set("Accept", "application/json, text/event-stream")
							req.Header.Set("Authorization", tc.header)
							rec := httptest.NewRecorder()
							h.ServeHTTP(rec, req)
							require.Equal(t, tc.want, rec.Code, "%s %s", path, method)
						}
					}
				})
			}
		})
	}
}

func TestAuthenticatedClientsReadResourcesConcurrently(t *testing.T) {
	t.Setenv("SLACK_MCP_API_KEY", "local-test-key")
	s := &MCPServer{server: server.NewMCPServer("auth-test", "1.0.0"), logger: zap.NewNop()}
	s.server.AddResource(mcp.NewResource("test://directory", "test directory"),
		func(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: "test://directory", Text: "synthetic directory"}}, nil
		})
	host := httptest.NewServer(s.ServeHTTP("").Handler)
	defer host.Close()
	errors := make(chan error, 2)
	for range 2 {
		go func() {
			call := func(body, session string) (string, string, error) {
				req, err := http.NewRequest(http.MethodPost, host.URL+"/mcp", strings.NewReader(body))
				if err != nil {
					return "", "", err
				}
				req.Header.Set("Authorization", "Bearer local-test-key")
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", "application/json, text/event-stream")
				if session != "" {
					req.Header.Set("Mcp-Session-Id", session)
				}
				c := &http.Client{Timeout: 5 * time.Second}
				res, err := c.Do(req)
				if err != nil {
					return "", "", err
				}
				defer res.Body.Close()
				data, err := io.ReadAll(res.Body)
				if res.StatusCode != http.StatusOK {
					return "", "", fmt.Errorf("status %d: %s", res.StatusCode, data)
				}
				return string(data), res.Header.Get("Mcp-Session-Id"), err
			}
			_, session, err := call(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, "")
			if err == nil && session == "" {
				err = fmt.Errorf("initialize did not create a session")
			}
			if err == nil {
				var body string
				body, _, err = call(`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"test://directory"}}`, session)
				if err == nil && !strings.Contains(body, "synthetic directory") {
					err = fmt.Errorf("resource contents missing: %s", body)
				}
			}
			errors <- err
		}()
	}
	for range 2 {
		require.NoError(t, <-errors)
	}
}
