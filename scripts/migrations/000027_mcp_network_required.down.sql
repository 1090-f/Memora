-- 000027_mcp_network_required.down.sql

ALTER TABLE mcp_servers
    DROP COLUMN IF EXISTS network_required;