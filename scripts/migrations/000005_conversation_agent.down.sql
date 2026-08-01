DROP TABLE IF EXISTS tool_calls;
DROP TABLE IF EXISTS agent_plan_steps;
DROP TABLE IF EXISTS agent_plans;
ALTER TABLE IF EXISTS messages DROP CONSTRAINT IF EXISTS fk_messages_agent_run;
DROP TABLE IF EXISTS agent_runs;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversations;
