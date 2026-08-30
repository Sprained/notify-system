CREATE TABLE topic (
    name        TEXT PRIMARY KEY,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE message (
    id         TEXT PRIMARY KEY,
    topic      TEXT NOT NULL REFERENCES topic(name),
    title      TEXT,
    body       TEXT NOT NULL,
    priority   SMALLINT NOT NULL CHECK (priority BETWEEN 1 AND 5),
    tags       TEXT[],
    click_url  TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_message_topic_id ON message (topic, id DESC);

CREATE TABLE subscriber (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind      TEXT NOT NULL CHECK (kind IN ('telegram', 'fcm', 'webpush')),
    config    JSONB NOT NULL,
    label     TEXT,
    enabled   BOOLEAN NOT NULL DEFAULT true,
    last_seen TIMESTAMPTZ
);

CREATE TABLE route (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    topic         TEXT NOT NULL REFERENCES topic(name),
    subscriber_id BIGINT NOT NULL REFERENCES subscriber(id),
    min_priority  SMALLINT NOT NULL CHECK (min_priority BETWEEN 1 AND 5),
    enabled       BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE delivery (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    message_id    TEXT NOT NULL REFERENCES message(id),
    subscriber_id BIGINT NOT NULL REFERENCES subscriber(id),
    status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    attempts      SMALLINT NOT NULL DEFAULT 0,
    last_error    TEXT,
    sent_at       TIMESTAMPTZ,
    UNIQUE (message_id, subscriber_id)
);

CREATE INDEX idx_delivery_failed ON delivery (subscriber_id) WHERE status = 'failed';

CREATE TABLE message_read (
    message_id    TEXT NOT NULL REFERENCES message(id),
    subscriber_id BIGINT NOT NULL REFERENCES subscriber(id),
    read_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, subscriber_id)
);
