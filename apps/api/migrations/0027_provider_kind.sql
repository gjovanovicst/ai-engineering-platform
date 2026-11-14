-- +goose Up
ALTER TABLE ai_provider_configs
    ADD COLUMN kind text NOT NULL DEFAULT 'openai' CHECK (kind IN ('openai','anthropic'));

-- +goose Down
ALTER TABLE ai_provider_configs DROP COLUMN kind;
