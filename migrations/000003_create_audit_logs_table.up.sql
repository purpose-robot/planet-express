CREATE TABLE plnx_audit_logs (
    id            UUID        PRIMARY KEY,
    created_at    TIMESTAMPTZ NOT NULL,
    action        TEXT        NOT NULL,
    user_id       UUID        REFERENCES plnx_users(id) ON DELETE SET NULL,
    resource_id   UUID        NOT NULL,
    resource_type TEXT        NOT NULL,
    status        TEXT        NOT NULL,
    metadata      JSONB,
    error_code    TEXT,

    CONSTRAINT plnx_audit_logs_status_check CHECK (status IN ('success', 'failure')),
    CONSTRAINT plnx_audit_logs_action_check CHECK (action IN ('created', 'updated', 'deleted', 'executed', 'authenticated', 'activated', 'password_reset', 'activation_requested', 'password_reset_requested'))
);
