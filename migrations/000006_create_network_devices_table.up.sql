CREATE TABLE plnx_network_devices (
    id                     UUID        PRIMARY KEY,
    created_at             TIMESTAMPTZ NOT NULL,
    updated_at             TIMESTAMPTZ NOT NULL,
    ip_address             INET        NOT NULL UNIQUE,
    hostname               TEXT,
    serial_number          TEXT,
    product_id             TEXT,
    software_version       TEXT,
    last_sync_status       TEXT        NOT NULL,
    last_sync_reachable    BOOLEAN,
    last_sync_attempted_at TIMESTAMPTZ,

    CONSTRAINT plnx_network_devices_last_sync_status_check CHECK (last_sync_status IN ('pending', 'success', 'failure'))
);
