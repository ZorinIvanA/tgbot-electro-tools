-- 005_fix_message_count_nulls.sql
-- Fix NULL message_count values for existing users

-- Ensure message_count has a default of 0 if it somehow doesn't
ALTER TABLE users ALTER COLUMN message_count SET DEFAULT 0;

-- Update existing rows where message_count might be NULL to 0
UPDATE users SET message_count = 0, updated_at = NOW() WHERE message_count IS NULL;

-- Add a NOT NULL constraint if it's not already enforced (information_schema says it is, but double check)
-- This is more of a safeguard and to make the intent explicit.
-- ALTER TABLE users ALTER COLUMN message_count SET NOT NULL;
-- Note: The above ALTER TABLE ... SET NOT NULL might fail if there are still NULLs,
-- but the UPDATE above should handle that. The information_schema already shows it as NOT NULL.
