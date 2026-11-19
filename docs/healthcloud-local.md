# HealthCloud local Slack MCP trial

This fork provides a shared Streamable HTTP endpoint for agents. Slack credentials stay in the server process; clients use a separate MCP bearer key. The Go module path remains upstream's path to keep the fork small.

## Start locally

Build with Go 1.27.4 or newer:

```sh
go build -o ./build/slack-mcp-server ./cmd/slack-mcp-server
```

Inject these variables from your credential manager or a private launcher:

| Variable | Value |
| --- | --- |
| `SLACK_MCP_XOXP_TOKEN` | User OAuth token for the intended workspace |
| `SLACK_MCP_API_KEY` | Independently generated random bearer key |
| `SLACK_MCP_HOST` | `127.0.0.1` |
| `SLACK_MCP_PORT` | `13080` |
| `SLACK_MCP_ENABLED_TOOLS` | `channels_list,users_search,conversations_history,conversations_replies,conversations_search_messages,conversations_add_message` |
| `SLACK_MCP_ADD_MESSAGE_TOOL` | `true`, or a comma-separated list of permitted channel IDs |

```sh
./build/slack-mcp-server --transport http
```

Clients connect to `http://127.0.0.1:13080/mcp` and send `Authorization: Bearer <MCP key>`. Give clients the MCP key, never the Slack token. Both HTTP and SSE reject every unauthenticated request; a missing configured key returns 503. Stdio does not require the network key. The legacy `SLACK_MCP_SSE_API_KEY` remains supported.

The token needs the existing directory/history scopes plus `search:read` for search and `chat:write` for sending. After adding scopes, reinstall the Slack app and verify the installed token's scopes. Slack may update the existing token's permissions; replace the local token only if Slack issues a different one. A successful token check alone does not prove these scopes. The local trial reuses Pi's credential loader and private secret file, without committing or duplicating the token. Restart the service after replacing a static token.

Use your own Slack DM for a clearly labeled test message. Verify its returned timestamp, read the resulting thread from another MCP client, and search for its unique marker. This is request/response read and write access; it does not subscribe to Slack Events or deliver unsolicited Slack messages to agents. Remote hosting and an event listener are separate work.

## Security and verification

Dependabot covers Go modules, GitHub Actions, and Docker base images weekly. Repository alerts and automated security updates must also be enabled in GitHub settings. The security workflow runs weekly, on pull requests, and on default-branch pushes.

The initial Go scan found 43 advisories, with call paths for 12. Updating the vulnerable modules and Go toolchain removes all fixable findings reported by that scan. `GO-2026-6911` remains in `github.com/mark3labs/mcp-go`: its HTTP client may forward credentials across redirects, and the advisory has no published fixed version. The scan reports no reachable path in this server. This is an outstanding dependency risk, not a clean module scan. Track the upstream fix at <https://github.com/mark3labs/mcp-go/issues/1164>.

GitHub also reports three advisories against `go-jose/v3`, which Playwright imports through the legacy `json` package. This fork explicitly resolves that module to the maintained `go-jose/v4` implementation at v4.1.9. The three advisories are patched in v4.1.5/v4.1.6; v3 has no published patch. No go-jose cryptography package is imported by this repository's packages. Remove this compatibility replacement when slackdump migrates its browser dependency to the newer Playwright module path. The local trial verifies User OAuth authentication; interactive browser authentication is not qualified by that trial.

```sh
go test -race -count=1 -skip '^TestIntegration(ChannelsListQueryFilter|PublicChannelsList|PrivateChannelsList|Conversations)$' ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./...
```

The four excluded upstream live tests require Slack credentials, ngrok, and/or an OpenAI API key. Run them only deliberately: their harness enables messaging and can invoke paid model calls. The GitHub live integration workflow is manual. Local protocol, authentication, resource, and other offline integration tests still run in CI.

Container dependency findings from the filesystem scan do not establish that a built container is vulnerability-free. The local trial uses the freshly compiled binary; no image or npm package is published.
