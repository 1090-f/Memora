ALTER TABLE agent_runs
    ADD COLUMN first_token_at timestamptz,
    ADD COLUMN first_token_latency_ms bigint CHECK (first_token_latency_ms IS NULL OR first_token_latency_ms >= 0),
    ADD COLUMN model_generate_duration_ms bigint CHECK (model_generate_duration_ms IS NULL OR model_generate_duration_ms >= 0),
    ADD COLUMN failure_stage varchar(40),
    ADD COLUMN retryable boolean,
    ADD COLUMN recovery_advice varchar(1000);
