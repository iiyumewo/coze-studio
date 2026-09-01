-- Create "skill" table
CREATE TABLE `opencoze`.`skill` (
  `id` bigint unsigned NOT NULL DEFAULT 0 COMMENT "Skill ID",
  `space_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT "Space ID",
  `creator_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT "Creator User ID",
  `name` varchar(128) NOT NULL DEFAULT "" COMMENT "Skill name from SKILL.md",
  `description` varchar(2000) NOT NULL DEFAULT "" COMMENT "Skill description from SKILL.md",
  `file_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT "Uploaded archive file ID",
  `file_name` varchar(255) NOT NULL DEFAULT "" COMMENT "Uploaded archive name",
  `file_size` bigint unsigned NOT NULL DEFAULT 0 COMMENT "Uploaded archive size",
  `entry_count` int unsigned NOT NULL DEFAULT 0 COMMENT "Archive entry count",
  `created_at` bigint unsigned NOT NULL DEFAULT 0 COMMENT "Create Time in Milliseconds",
  `updated_at` bigint unsigned NOT NULL DEFAULT 0 COMMENT "Update Time in Milliseconds",
  `deleted_at` datetime(3) NULL COMMENT "Delete Time",
  PRIMARY KEY (`id`),
  INDEX `idx_skill_space_updated_at` (`space_id`, `updated_at`),
  INDEX `idx_skill_creator_id` (`creator_id`)
) CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci COMMENT "Workspace Skill resource";
