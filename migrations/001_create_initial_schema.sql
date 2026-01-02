-- 001_create_initial_schema.sql
-- Initial database schema for Telegram bot

-- Settings table (single row configuration)
CREATE TABLE IF NOT EXISTS settings (
    id SERIAL PRIMARY KEY,
    trigger_message_count INT NOT NULL DEFAULT 3,
    site_url TEXT NOT NULL DEFAULT 'https://example.com',
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Insert default settings
INSERT INTO settings (trigger_message_count, site_url)
VALUES (3, 'https://example.com')
ON CONFLICT (id) DO NOTHING;

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT NOT NULL UNIQUE,
    username TEXT,
    first_name TEXT,
    last_name TEXT,
    message_count INT NOT NULL DEFAULT 0,
    fsm_state TEXT NOT NULL DEFAULT 'idle',
    email TEXT,
    consent_granted BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create index on telegram_id for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);

-- Create index on fsm_state for metrics
CREATE INDEX IF NOT EXISTS idx_users_fsm_state ON users(fsm_state);

-- Messages log table (for metrics and debugging)
CREATE TABLE IF NOT EXISTS messages (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
    message_text TEXT,
    direction TEXT NOT NULL CHECK (direction IN ('incoming', 'outgoing')),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Create index on created_at for time-based queries (24h metrics)
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_messages_user_id ON messages(user_id);

-- Rate limiting table (stores last message timestamps)
CREATE TABLE IF NOT EXISTS rate_limits (
    telegram_id BIGINT PRIMARY KEY,
    message_timestamps BIGINT[], -- Array of Unix timestamps
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Сценарии (например: "ушм_не_включается")
CREATE TABLE fsm_scenarios (
    id SERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,          -- "ushm_not_starting"
    trigger_keywords TEXT[] NOT NULL,   -- ["не включается", "молчит"]
    description TEXT                     -- для админки
);

-- Шаги сценария
CREATE TABLE fsm_steps (
    id SERIAL PRIMARY KEY,
    scenario_id INT REFERENCES fsm_scenarios(id) ON DELETE CASCADE,
    step_key TEXT NOT NULL,             -- "step1", "step2"
    message TEXT NOT NULL,              -- текст ответа бота
    is_final BOOL DEFAULT false,
    next_step_key TEXT,                 -- NULL = завершение
    UNIQUE(scenario_id, step_key)
);

-- Сессии пользователей
CREATE TABLE user_sessions (
    user_id BIGINT PRIMARY KEY,
    current_scenario_id INT REFERENCES fsm_scenarios(id),
    current_step_key TEXT,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Вставка сценариев диагностики
INSERT INTO fsm_scenarios (name, trigger_keywords, description) VALUES
('ushm_not_starting', ARRAY['не включается', 'не запускается', 'молчит', 'не жужжит', 'не крутит'], 'Диагностика проблемы запуска угловой шлифовальной машины');

-- Вставка шагов для сценария "ushm_not_starting"
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
(1, 'step1', 'Понял, проблема с запуском. Что именно происходит? Опишите, пожалуйста, подробнее: устройство совсем не реагирует на нажатие кнопки, или есть какие-то звуки, индикация?', false, 'step2'),
(1, 'step2', 'Давайте попробуем продиагностировать проблему:

1. Проверьте, нажимаете ли вы рычажок предохранителя (обычно находится на корпусе)
2. Убедитесь, что розетка работает (проверьте другим устройством)
3. Осмотрите кабель на наличие повреждений
4. Если есть кнопка блокировки шпинделя - убедитесь, что она не зажата

Проверьте эти моменты и напишите результат.', true, NULL);
