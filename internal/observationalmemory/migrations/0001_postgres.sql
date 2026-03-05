CREATE TABLE IF NOT EXISTS om_memory_records (
	memory_id TEXT PRIMARY KEY,
	tenant_id TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	agent_id TEXT NOT NULL,
	enabled BOOLEAN NOT NULL,
	state_version BIGINT NOT NULL,
	last_observed_message_index BIGINT NOT NULL,
	active_observations_json TEXT NOT NULL,
	active_observation_tokens BIGINT NOT NULL,
	active_reflection TEXT NOT NULL,
	active_reflection_tokens BIGINT NOT NULL,
	last_reflected_observation_seq BIGINT NOT NULL,
	config_json TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	UNIQUE (tenant_id, conversation_id, agent_id)
);

CREATE TABLE IF NOT EXISTS om_operation_log (
	operation_id TEXT PRIMARY KEY,
	memory_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	tool_call_id TEXT NOT NULL,
	scope_sequence BIGINT NOT NULL,
	operation_type TEXT NOT NULL,
	status TEXT NOT NULL,
	payload_json TEXT NOT NULL,
	error_text TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS om_markers (
	marker_id TEXT PRIMARY KEY,
	memory_id TEXT NOT NULL,
	marker_type TEXT NOT NULL,
	cycle_id TEXT NOT NULL,
	message_index_start BIGINT NOT NULL,
	message_index_end BIGINT NOT NULL,
	token_count BIGINT NOT NULL,
	payload_json TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS om_operation_log_memory_scope_seq_idx ON om_operation_log(memory_id, scope_sequence);
CREATE INDEX IF NOT EXISTS om_operation_log_status_created_idx ON om_operation_log(status, created_at);
CREATE INDEX IF NOT EXISTS om_markers_memory_created_idx ON om_markers(memory_id, created_at);
