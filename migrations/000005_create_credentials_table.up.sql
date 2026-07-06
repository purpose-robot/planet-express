CREATE TABLE plnx_credentials (
    id                 UUID        PRIMARY KEY,
    created_at         TIMESTAMPTZ NOT NULL,
    updated_at         TIMESTAMPTZ NOT NULL,
    user_id            UUID        REFERENCES plnx_users (id) ON DELETE CASCADE,
    username           TEXT        NOT NULL,
    encrypted_password BYTEA       NOT NULL,
    auth_method        TEXT        NOT NULL,
    description        TEXT,
    last_used_at       TIMESTAMPTZ,

    CONSTRAINT plnx_credentials_auth_method_check CHECK (auth_method IN ('local', 'remote'))
);

CREATE UNIQUE INDEX plnx_credentials_auth_method_local_unique ON plnx_credentials (auth_method)
    WHERE auth_method = 'local';
