-- Modify "single_agent_draft" table
ALTER TABLE `opencoze`.`single_agent_draft` ADD COLUMN `mcp` json NULL COMMENT "Agent MCP Server Configuration" AFTER `plugin`;

-- Modify "single_agent_version" table
ALTER TABLE `opencoze`.`single_agent_version` ADD COLUMN `mcp` json NULL COMMENT "Agent MCP Server Configuration" AFTER `plugin`;
