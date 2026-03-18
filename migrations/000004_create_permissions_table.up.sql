CREATE TABLE plnx_permissions (
    id   BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    code TEXT   NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS plnx_users_permissions (
    user_id       UUID   NOT NULL REFERENCES plnx_users (id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES plnx_permissions ON DELETE CASCADE,
    PRIMARY KEY (user_id, permission_id)
);

INSERT INTO plnx_permissions (code) VALUES
    ('health:read'),
    ('credentials:read'),
    ('credentials:write'),
    ('network-devices:read'),
    ('network-devices:write');
