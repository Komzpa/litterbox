CREATE TABLE tenants (
    id uuid PRIMARY KEY,
    next_seq bigint NOT NULL DEFAULT 0 CHECK (next_seq >= 0),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE devices (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id uuid NOT NULL,
    token_hash bytea NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, token_hash)
);

CREATE TABLE invites (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id uuid NOT NULL,
    code_hash bytea NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, code_hash)
);

CREATE TABLE accounts (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id uuid NOT NULL,
    address text NOT NULL,
    refresh_token bytea NOT NULL,
    history_id text,
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, address)
);

CREATE TABLE bundles (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id uuid NOT NULL,
    title text NOT NULL,
    centroid real[] NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id)
);

CREATE TABLE cards (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id uuid NOT NULL,
    account_id uuid NOT NULL,
    gmail_thread_id text NOT NULL,
    state text NOT NULL DEFAULT 'open'
        CHECK (state IN ('open', 'snoozed', 'archived', 'done')),
    bundle_id uuid,
    pinned_rank bigint,
    snooze_until timestamptz,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    subject text NOT NULL DEFAULT '',
    sender text NOT NULL DEFAULT '',
    sort_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, account_id, gmail_thread_id),
    FOREIGN KEY (tenant_id, account_id)
        REFERENCES accounts (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, bundle_id)
        REFERENCES bundles (tenant_id, id) ON DELETE SET NULL (bundle_id)
);

CREATE TABLE messages (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id uuid NOT NULL,
    card_id uuid NOT NULL,
    gmail_message_id text NOT NULL,
    labels text[] NOT NULL DEFAULT '{}',
    html text NOT NULL DEFAULT '',
    text text NOT NULL DEFAULT '',
    body_hash bytea NOT NULL,
    received_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, card_id, gmail_message_id),
    FOREIGN KEY (tenant_id, card_id)
        REFERENCES cards (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE exclusions (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id uuid NOT NULL,
    bundle_id uuid NOT NULL,
    card_id uuid NOT NULL,
    embedding real[] NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, bundle_id, card_id),
    FOREIGN KEY (tenant_id, bundle_id)
        REFERENCES bundles (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, card_id)
        REFERENCES cards (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE ops (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    op_id uuid NOT NULL,
    device_id uuid NOT NULL,
    payload_hash bytea NOT NULL,
    payload jsonb NOT NULL,
    result jsonb,
    cursor bigint,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, op_id),
    FOREIGN KEY (tenant_id, device_id)
        REFERENCES devices (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE gmail_effects (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id uuid NOT NULL,
    op_id uuid NOT NULL,
    account_id uuid NOT NULL,
    message_id uuid NOT NULL,
    add_labels text[] NOT NULL DEFAULT '{}',
    remove_labels text[] NOT NULL DEFAULT '{}',
    state text NOT NULL DEFAULT 'pending',
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, op_id, account_id, message_id),
    FOREIGN KEY (tenant_id, op_id)
        REFERENCES ops (tenant_id, op_id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, account_id)
        REFERENCES accounts (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, message_id)
        REFERENCES messages (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE changes (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    seq bigint NOT NULL CHECK (seq > 0),
    entity text NOT NULL CHECK (entity IN ('account', 'bundle', 'card', 'message')),
    id uuid NOT NULL,
    value jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, seq)
);

CREATE INDEX cards_open_list_idx
    ON cards (tenant_id, pinned_rank, sort_at DESC)
    WHERE state = 'open';
CREATE INDEX cards_bundle_idx ON cards (tenant_id, bundle_id);
CREATE INDEX messages_card_idx ON messages (tenant_id, card_id);
CREATE INDEX exclusions_card_idx ON exclusions (tenant_id, card_id);
CREATE INDEX ops_device_idx ON ops (tenant_id, device_id);
CREATE INDEX gmail_effects_account_idx ON gmail_effects (tenant_id, account_id);
CREATE INDEX gmail_effects_message_idx ON gmail_effects (tenant_id, message_id);
