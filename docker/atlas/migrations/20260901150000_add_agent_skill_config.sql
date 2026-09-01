-- Modify "single_agent_draft" table
ALTER TABLE `opencoze`.`single_agent_draft` ADD COLUMN `skill` json NULL COMMENT "Agent Skill Configuration" AFTER `mcp`;

-- Modify "single_agent_version" table
ALTER TABLE `opencoze`.`single_agent_version` ADD COLUMN `skill` json NULL COMMENT "Agent Skill Configuration" AFTER `mcp`;
