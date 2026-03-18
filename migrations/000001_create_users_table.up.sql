CREATE TABLE plnx_users (
    id                UUID        PRIMARY KEY,
    created_at        TIMESTAMPTZ NOT NULL,
    updated_at        TIMESTAMPTZ NOT NULL,
    activated         BOOLEAN     NOT NULL,
    encrypted_email   BYTEA       NOT NULL,
    hashed_password   TEXT        NOT NULL,
    email_blind_index TEXT        NOT NULL UNIQUE
);
