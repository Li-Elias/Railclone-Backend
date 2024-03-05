CREATE TABLE IF NOT EXISTS deployments (
    id bigserial PRIMARY KEY,
    image text NOT NULL, 
    port int NOT NULL,
    volume int NOT NULL,
    replicas int NOT NULL,
    env_vars text NOT NULL,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    last_updated timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    user_id bigint NOT NULL REFERENCES users ON DELETE CASCADE,
    running boolean NOT NULL
);
