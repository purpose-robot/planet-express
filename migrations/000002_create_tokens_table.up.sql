CREATE TABLE plnx_tokens (
    id      UUID        PRIMARY KEY,
    hash    BYTEA       NOT NULL UNIQUE,
    scope   TEXT        NOT NULL,
    user_id UUID        NOT NULL REFERENCES plnx_users (id) ON DELETE CASCADE,
    expiry  TIMESTAMPTZ NOT NULL
);
