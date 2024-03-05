CREATE EXTENSION citext;

CREATE TABLE IF NOT EXISTS users (
    id bigserial PRIMARY KEY,
    email citext UNIQUE NOT NULL,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    last_updated timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    activated bool NOT NULL
);