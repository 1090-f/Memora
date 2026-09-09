ALTER TABLE agent_runs
    DROP COLUMN IF EXISTS recovery_advice,
    DROP COLUMN IF EXISTS retryable,
    DROP COLUMN IF EXISTS failure_stage,
    DROP COLUMN IF EXISTS model_generate_duration_ms,
    DROP COLUMN IF EXISTS first_token_latency_ms,
    DROP COLUMN IF EXISTS first_token_at;
