-- 006_set_default_trigger_message_count.sql
-- Ensure trigger_message_count in settings table is set correctly
-- Using a default of 4 if it's NULL or 0, matching typical .env configuration

-- Update the specific settings row (id=1)
UPDATE settings
SET trigger_message_count = 4,
    site_url = COALESCE(site_url, 'https://example.com'), -- Also ensure site_url has a default if NULL
    updated_at = NOW()
WHERE id = 1
AND (trigger_message_count IS NULL OR trigger_message_count = 0 OR trigger_message_count != 4); -- Condition to update

-- If the row with id=1 doesn't exist, insert it with default values.
-- This handles the case where the settings table might be empty.
INSERT INTO settings (id, trigger_message_count, site_url, updated_at)
VALUES (1, 4, 'https://example.com', NOW())
ON CONFLICT (id) DO NOTHING; -- Only insert if id=1 does not exist

-- Ensure future inserts/updates that don't specify a value for trigger_message_count
-- will default to 4.
-- This is more of a safeguard for application logic.
-- ALTER TABLE settings ALTER COLUMN trigger_message_count SET DEFAULT 4;