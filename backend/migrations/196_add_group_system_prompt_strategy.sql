ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS system_prompt_strategy TEXT NOT NULL DEFAULT 'append';
