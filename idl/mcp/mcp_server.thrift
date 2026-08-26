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

struct GetMCPServerRequest {
    1: required i64 MCPServerID (agw.js_conv = "str", api.js_conv = "true", api.path = "mcp_server_id")
    2: required i64 SpaceID (agw.js_conv = "str", api.js_conv = "true", api.query = "space_id")

    255: base.Base Base (api.none = "true")
}

struct MCPServerAuthenticationData {
    1: required string Type (api.body = "type")
    2: optional string HeaderName (api.body = "header_name")
}

struct MCPToolData {
    1: required string Name (api.body = "name")
    2: optional string Title (api.body = "title")
    3: optional string Description (api.body = "description")
}

struct MCPServerDetailData {
    1: required i64 ID (agw.js_conv = "str", api.js_conv = "true", api.body = "id")
    2: required string Name (api.body = "name")
    3: optional string Description (api.body = "description")
    4: required string ServerURL (api.body = "server_url")
    5: required string Transport (api.body = "transport")
    6: required MCPServerAuthenticationData Authentication (api.body = "authentication")
    7: required list<MCPToolData> Tools (api.body = "tools")
    8: optional string ProtocolVersion (api.body = "protocol_version")
    9: optional string ServerVersion (api.body = "server_version")
    10: required i32 Status (api.body = "status")
    11: required string ValidationStatus (api.body = "validation_status")
    12: optional string LastError (api.body = "last_error")
    13: required i64 LastCheckedAt (api.body = "last_checked_at")
    14: required i64 CreatedAt (api.body = "created_at")
    15: required i64 UpdatedAt (api.body = "updated_at")
}

struct GetMCPServerResponse {
    1: optional MCPServerDetailData Data (api.body = "data")

    253: required i64 Code (api.body = "code")
    254: required string Msg (api.body = "msg")
    255: required base.BaseResp BaseResp
}

struct DeleteMCPServerRequest {
    1: required i64 MCPServerID (agw.js_conv = "str", api.js_conv = "true", api.path = "mcp_server_id")
    2: required i64 SpaceID (agw.js_conv = "str", api.js_conv = "true", api.query = "space_id")

    255: base.Base Base (api.none = "true")
}

struct DeleteMCPServerResponse {
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

    GetMCPServerResponse GetMCPServer(
        1: GetMCPServerRequest request
    ) (
        api.get = "/api/mcp_manage/v1/mcp_servers/:mcp_server_id",
        api.category = "mcp_manage",
        agw.preserve_base = "true"
    )

    DeleteMCPServerResponse DeleteMCPServer(
        1: DeleteMCPServerRequest request
    ) (
        api.delete = "/api/mcp_manage/v1/mcp_servers/:mcp_server_id",
        api.category = "mcp_manage",
        agw.preserve_base = "true"
    )
}
