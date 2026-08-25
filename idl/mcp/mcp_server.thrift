include "../base.thrift"

namespace go mcp

struct MCPServerAuthentication {
    1: required string Type (api.body = "type")
    2: optional string HeaderName (api.body = "header_name")
    3: optional string HeaderValue (api.body = "header_value")
}

struct TestAndCreateMCPServerRequest {
    1: required i64 SpaceID (agw.js_conv = "str", api.js_conv = "true", api.body = "space_id")
    2: optional string Name (api.body = "name")
    3: required string ServerURL (api.body = "server_url")
    4: required string Transport (api.body = "transport")
    5: required MCPServerAuthentication Authentication (api.body = "authentication")

    255: base.Base Base (api.none = "true")
}

struct MCPServerData {
    1: required i64 ID (agw.js_conv = "str", api.js_conv = "true", api.body = "id")
    2: required string Name (api.body = "name")
    3: required i32 ToolCount (api.body = "tool_count")
}

struct TestAndCreateMCPServerResponse {
    1: optional MCPServerData Data (api.body = "data")

    253: required i64 Code (api.body = "code")
    254: required string Msg (api.body = "msg")
    255: required base.BaseResp BaseResp
}

service MCPManageService {
    TestAndCreateMCPServerResponse TestAndCreateMCPServer(
        1: TestAndCreateMCPServerRequest request
    ) (
        api.post = "/api/mcp_manage/v1/mcp_servers/test_and_create",
        api.category = "mcp_manage",
        agw.preserve_base = "true"
    )
}
