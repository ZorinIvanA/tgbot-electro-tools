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
CREATE TABLE IF NOT EXISTS fsm_scenarios (
    id SERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,          -- "ushm_not_starting"
    trigger_keywords TEXT[] NOT NULL,   -- ["не включается", "молчит"]
    description TEXT                     -- для админки
);

-- Шаги сценария
CREATE TABLE IF NOT EXISTS fsm_steps (
    id SERIAL PRIMARY KEY,
    scenario_id INT REFERENCES fsm_scenarios(id) ON DELETE CASCADE,
    step_key TEXT NOT NULL,             -- "step1", "step2"
    message TEXT NOT NULL,              -- текст ответа бота
    is_final BOOL DEFAULT false,
    next_step_key TEXT,                 -- NULL = завершение
    UNIQUE(scenario_id, step_key)
);

-- Сессии пользователей
CREATE TABLE IF NOT EXISTS user_sessions (
    user_id BIGINT PRIMARY KEY,
    current_scenario_id INT REFERENCES fsm_scenarios(id),
    current_step_key TEXT,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Очистка старых данных FSM (если миграция перезапускается)
-- Учитываем ограничения внешнего ключа: сначала дочерние таблицы, затем родительские
TRUNCATE TABLE user_sessions CASCADE;
TRUNCATE TABLE fsm_steps CASCADE;
TRUNCATE TABLE fsm_scenarios CASCADE;

-- Сброс последовательностей ID для таблиц FSM
ALTER SEQUENCE fsm_scenarios_id_seq RESTART WITH 1;
ALTER SEQUENCE fsm_steps_id_seq RESTART WITH 1;

-- Вставка сценариев диагностики УШМ
INSERT INTO fsm_scenarios (name, trigger_keywords, description) VALUES
('ushm_not_starting', ARRAY['не включается', 'не запускается', 'молчит', 'не жужжит', 'не крутит', 'не реагирует на кнопку'], 'Диагностика проблемы запуска угловой шлифовальной машины'),
('ushm_stopped_during_work', ARRAY['остановилась', 'перестала работать', 'выключилась', 'заглохла'], 'Диагностика внезапной остановки угловой шлифовальной машины'),
('battery_charge', ARRAY['аккумулятор разряжен', 'низкий заряд', 'аккумулятор полностью разрядился'], 'Зарядка или замена аккумулятора'),
('replace_button', ARRAY['кнопка включения', 'кнопка повреждена', 'залипание кнопки'], 'Замена кнопки включения'),
('repair_connection', ARRAY['обрыв цепи', 'отпайка провода', 'плохой контакт'], 'Ремонт электрического соединения'),
('replace_motor', ARRAY['сгорел электродвигатель', 'обмотка в обрыве'], 'Замена двигателя или обращение в сервис'),
('restart_device', ARRAY['повторный запуск', 'остыть', 'дать остыть'], 'Повторный запуск устройства'),
('repair_mechanics', ARRAY['заклинивание редуктора', 'поломка шестерён', 'выход из строя подшипников'], 'Ремонт механической части'),
('replace_brushes', ARRAY['износ щёток', 'искрение', 'нестабильная работа'], 'Замена угольных щёток'),
('service_center', ARRAY['неисправность электроники', 'требуется диагностика', 'обращение в сервис'], 'Обращение в сервисный центр'),
('diagnose_miter_saw', ARRAY['торцовочная пила', 'пила торцовочная', 'торцовка', 'miter saw'], 'Диагностика торцовочной пилы'),
('diagnose_jigsaw', ARRAY['электролобзик', 'лобзик', 'jigsaw'], 'Диагностика электролобзика'),
('diagnose_cordless_screwdriver', ARRAY['аккумуляторный шуруповёрт', 'шуруповёрт', 'cordless screwdriver'], 'Диагностика аккумуляторного шуруповёрта'),
('diagnose_corded_lawn_mower', ARRAY['проводная газонокосилка', 'газонокосилка', 'lawn mower'], 'Диагностика проводной газонокосилки');

-- Вставка шагов для сценария "ushm_not_starting" (ID=1)
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
(1, 'step1', 'Понял, проблема с запуском УШМ. Давайте определим причину:

Возможные причины:
1. Аккумулятор разряжен
2. Неисправность кнопки включения (залипание от пыли/грязи)
3. Обрыв цепи питания (вибрация, падение)
4. Сгорел электродвигатель

Опишите подробнее: устройство совсем не реагирует на кнопку, или есть признаки работы? Что вы уже пробовали?', false, 'step2'),
(1, 'step2', 'Рекомендации по диагностике:

• Проверьте уровень заряда аккумулятора
• Очистите кнопку включения от пыли и грязи
• Осмотрите провода на обрывы
• Если ничего не помогает - проверьте целостность обмоток двигателя мультиметром

Если проблема не решена - обратитесь в сервисный центр для профессиональной диагностики.', true, NULL);

-- Вставка шагов для сценария "ushm_stopped_during_work" (ID=2)
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
(2, 'step1', 'Понял, УШМ остановилась во время работы. Давайте определим причину:

Возможные причины:
1. Защитное отключение аккумулятора (превышение мощности)
2. Аккумулятор полностью разрядился
3. Сработала термозащита двигателя (перегрев)
4. Механическое заклинивание редуктора или двигателя
5. Износ угольных щёток
6. Неисправность электроники

Расскажите подробнее: были ли искры, перегрев, необычные звуки? Под нагрузкой остановилась?', false, 'step2'),
(2, 'step2', 'Рекомендации по диагностике:

• Снимите и повторно установите аккумулятор
• Дайте инструменту остыть 15–30 минут
• Проверьте, не заклинило ли диск вручную
• Осмотрите щётки на износ
• Если ничего не помогает - требуется диагностика электроники в сервисе

Опишите результаты проверки.', true, NULL);

-- Вставка шагов для сценария "battery_charge" (ID=3)
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
(3, 'step1', 'Проблема с аккумулятором. Действия:

1. Проверьте уровень заряда на индикаторе (если есть)
2. Подключите к зарядному устройству на 1-2 часа
3. Если зарядка не помогает - замените аккумулятор на заряженный
4. Рекомендуется использовать оригинальные аккумуляторы

После зарядки/замены попробуйте запустить инструмент.', true, NULL);

-- Вставка шагов для сценария "replace_button" (ID=4)
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
(4, 'step1', 'Неисправность кнопки включения. Действия:

1. Разберите корпус УШМ (осторожно, соблюдая технику безопасности)
2. Осмотрите кнопку на повреждения
3. Очистите от пыли и грязи
4. Если кнопка повреждена - замените на новую
5. Соберите инструмент обратно

Если не уверены в своих силах - обратитесь в сервис.', true, NULL);

-- Вставка шагов для сценария "repair_connection" (ID=5)
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
(5, 'step1', 'Обрыв цепи питания. Действия:

1. Разберите корпус УШМ
2. Найдите место обрыва провода (часто у аккумулятора)
3. Восстановите пайку повреждённого участка
4. Изолируйте соединение
5. Соберите и проверьте работу

Требуется навык пайки. Если нет опыта - в сервисный центр.', true, NULL);

-- Вставка шагов для сценария "replace_motor" (ID=6)
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
(6, 'step1', 'Сгорел электродвигатель. Действия:

1. Разберите УШМ и достаньте двигатель
2. Проверьте обмотки мультиметром на обрыв
3. Если обмотка оборвана - требуется замена двигателя
4. Замена двигателя - сложная процедура, лучше в сервисе

Рекомендуется профессиональный ремонт.', true, NULL);

-- Вставка шагов для сценария "restart_device" (ID=7)
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
(7, 'step1', 'Требуется повторный запуск. Действия:

1. Полностью отключите инструмент
2. Дайте остыть 15–30 минут (особенно после интенсивной работы)
3. Повторно подключите аккумулятор
4. Попробуйте запустить

Если проблема повторяется - ищите другие причины.', true, NULL);

-- Вставка шагов для сценария "repair_mechanics" (ID=8)
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
(8, 'step1', 'Механическое заклинивание. Действия:

1. Разберите редуктор УШМ
2. Осмотрите шестерни на износ/поломку
3. Проверьте подшипники (редуктора и двигателя)
4. Замените повреждённые детали
5. Соберите и смажьте

Требуется специальный инструмент. Рекомендуется сервис.', true, NULL);

-- Вставка шагов для сценария "replace_brushes" (ID=9)
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
(9, 'step1', 'Износ угольных щёток. Действия:

1. Разберите корпус УШМ
2. Найдите щётки (обычно 2 шт.)
3. Измерьте длину (если <5мм - заменить)
4. Замените щётки на новые
5. Иногда помогает лёгкое подгибание для лучшего контакта

После замены проверьте работу.', true, NULL);

-- Вставка шагов для сценария "service_center" (ID=10)
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
(10, 'step1', 'Требуется профессиональная диагностика. Рекомендации:

• Не разбирайте инструмент самостоятельно - можете повредить
• Обратитесь в авторизованный сервисный центр
• Возьмите с собой инструмент, зарядку, инструкцию
• Опишите мастеру все симптомы и обстоятельства поломки

Сервис определит точную причину и выполнит ремонт.', true, NULL);

-- Вставка шагов для сценария "diagnose_miter_saw"
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_miter_saw'), 'step1', 'Выберите проблему с торцовочной пилой:

1. Не включается
2. Пила включается, но не вращается диск
3. Пила работает, но вибрирует или режет неточно

Ответьте цифрой или опишите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_miter_saw'), 'step2_1', 'Не включается. Признак: Нажатие кнопки пуска не вызывает реакции.

Возможные причины:
1.1. Отсутствует питание
1.2. Неисправность сетевого кабеля
1.3. Защита от случайного пуска активна
1.4. Сгорел двигатель или пусковой конденсатор

Выберите под-причину (1.1, 1.2, etc.) или опишите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_miter_saw'), 'final_restore_power', 'Восстановить подачу питания: Проверьте вилку, розетку, удлинитель. Используйте мультиметр для проверки напряжения.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_miter_saw'), 'final_replace_cable', 'Замена сетевого кабеля: Визуальный осмотр + прозвонка. Причина: Перелом, перетирание, обрыв.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_miter_saw'), 'final_disable_lock', 'Отключить блокировку / правильно установить пилу: Проверить блокировки (часто механические). Причина: Не опущен защитный кожух или не зафиксирована рукоятка.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_miter_saw'), 'final_replace_motor', 'Замена двигателя или обращение в сервис: Проверить обмотки и конденсатор. Признак: Запах гари, отсутствие звука.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_miter_saw'), 'step2_2', 'Пила включается, но не вращается диск. Признак: Мотор гудит, но диск неподвижен.

Возможные причины:
2.1. Клин ременной передачи или шпинделя
2.2. Обрыв ремня

Выберите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_miter_saw'), 'final_repair_mechanics', 'Ремонт механической части: Разборка, очистка, проверка вращения вручную. Причина: Попадание опилок, деформация, износ подшипника.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_miter_saw'), 'final_replace_belt', 'Замена ремня: Визуальный осмотр.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_miter_saw'), 'step2_3', 'Пила работает, но вибрирует или режет неточно.

Возможные причины:
3.1. Неправильная установка диска
3.2. Изношенный или повреждённый диск
3.3. Люфт вала или износ подшипников

Выберите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_miter_saw'), 'final_reinstall_disk', 'Переустановить диск: Проверить затяжку гайки, совпадение посадочного отверстия.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_miter_saw'), 'final_replace_disk', 'Установка нового диска: Заменить диск.', true, NULL);

-- Вставка шагов для сценария "diagnose_jigsaw"
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_jigsaw'), 'step1', 'Выберите проблему с электролобзиком:

1. Не включается
2. Включается, но полотно не движется
3. Сильная вибрация / увод в сторону

Ответьте цифрой или опишите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_jigsaw'), 'step2_1', 'Не включается.

Возможные причины:
1.1. Проблема с питанием
1.2. Неисправность кнопки пуска
1.3. Обрыв внутри корпуса

Выберите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_jigsaw'), 'final_restore_power_jigsaw', 'Восстановить подачу питания: Проверьте розетку, кабель, УЗО.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_jigsaw'), 'final_replace_button_jigsaw', 'Замена кнопки пуска: Разборка, чистка или замена. Причина: Загрязнение, износ контактов.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_jigsaw'), 'final_repair_wiring', 'Ремонт внутренней проводки: Прозвонка цепи. Причина: Перегиб кабеля у входа, вибрация.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_jigsaw'), 'step2_2', 'Включается, но полотно не движется.

Возможные причины:
2.1. Износ или поломка штока/кривошипа
2.2. Полотно не зафиксировано

Выберите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_jigsaw'), 'final_repair_drive', 'Ремонт привода хода полотна: Разборка, осмотр механизма. Причина: Механический износ.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_jigsaw'), 'final_install_blade', 'Правильная установка полотна: Проверить зажим, установить корректно.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_jigsaw'), 'step2_3', 'Сильная вибрация / увод в сторону.

Возможные причины:
3.1. Тупое или неподходящее полотно
3.2. Износ направляющих роликов
3.3. Ослаблен крепёж корпуса

Выберите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_jigsaw'), 'final_replace_blade', 'Установка нового полотна: Заменить на подходящее по материалу.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_jigsaw'), 'final_repair_mechanics_jigsaw', 'Ремонт механической части: Замена роликов. Причина: Люфт, биение.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_jigsaw'), 'final_service_body', 'Обслуживание корпуса: Подтянуть винты.', true, NULL);

-- Вставка шагов для сценария "diagnose_cordless_screwdriver"
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_cordless_screwdriver'), 'step1', 'Выберите проблему с аккумуляторным шуруповёртом:

1. Не включается
2. Вращается, но не крутит (проскальзывает)
3. Аккумулятор быстро садится

Ответьте цифрой или опишите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_cordless_screwdriver'), 'step2_1', 'Не включается.

Возможные причины:
1.1. Аккумулятор разряжен
1.2. Неисправность кнопки реверса/пуска
1.3. Плохой контакт между аккумулятором и корпусом

Выберите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_cordless_screwdriver'), 'final_charge_battery', 'Зарядить/заменить аккумулятор: Проверить индикатор, зарядить.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_cordless_screwdriver'), 'final_repair_button_screwdriver', 'Ремонт/замена кнопки: Разборка, чистка. Причина: Загрязнение, износ.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_cordless_screwdriver'), 'final_service_contacts', 'Обслуживание контактной группы: Очистить контакты, проверить фиксацию. Причина: Окисление, деформация контактов.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_cordless_screwdriver'), 'step2_2', 'Вращается, но не крутит (проскальзывает).

Возможные причины:
2.1. Сработала муфта регулировки крутящего момента
2.2. Износ редуктора (пластиковые шестерни)

Выберите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_cordless_screwdriver'), 'final_adjust_torque', 'Настройка крутящего момента: Увеличить номер на кольце крутящего момента.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_cordless_screwdriver'), 'final_repair_gearbox', 'Ремонт редуктора: Разборка, замена шестерён. Причина: Хруст, проскальзывание под нагрузкой.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_cordless_screwdriver'), 'step2_3', 'Аккумулятор быстро садится.

Возможные причины:
3.1. Износ элементов аккумулятора («эффект памяти» или деградация)
3.2. Неисправность платы BMS

Выберите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_cordless_screwdriver'), 'final_replace_battery_elements', 'Замена аккумулятора или его элементов: Проверить напряжение на ячейках.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_cordless_screwdriver'), 'final_diagnose_bms', 'Диагностика BMS / обращение в сервис: Причина: Раннее отключение, перегрев.', true, NULL);

-- Вставка шагов для сценария "diagnose_corded_lawn_mower"
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_corded_lawn_mower'), 'step1', 'Выберите проблему с проводной газонокосилкой:

1. Не включается
2. Включается, но нож не вращается
3. Неровный срез / вибрация

Ответьте цифрой или опишите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_corded_lawn_mower'), 'step2_1', 'Не включается.

Возможные причины:
1.1. Отсутствует питание
1.2. Обрыв кабеля
1.3. Сработала тепловая защита двигателя

Выберите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_corded_lawn_mower'), 'final_restore_power_lawn_mower', 'Восстановить подачу питания: Проверьте розетку, УЗО, целостность кабеля.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_corded_lawn_mower'), 'final_replace_cable_lawn_mower', 'Замена сетевого кабеля: Прозвонка. Причина: Перегиб, перетирание.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_corded_lawn_mower'), 'final_restart_after_cooling', 'Повторный запуск после остывания: Дать остыть 20–30 мин. Причина: Перегрев (долгая работа, высокая трава).', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_corded_lawn_mower'), 'step2_2', 'Включается, но нож не вращается.

Возможные причины:
2.1. Клин ножа
2.2. Обрыв ремня
2.3. Износ подшипников вала

Выберите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_corded_lawn_mower'), 'final_clean_area', 'Очистка рабочей зоны: Отключить от сети, очистить нож. Причина: Камень, ветка, намотанная трава.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_corded_lawn_mower'), 'final_replace_belt_lawn_mower', 'Замена ремня: Визуальный осмотр.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_corded_lawn_mower'), 'final_repair_mechanics_lawn_mower', 'Ремонт механической части: Признак: Скрип, трудное вращение вручную.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_corded_lawn_mower'), 'step2_3', 'Неровный срез / вибрация.

Возможные причины:
3.1. Тупой или повреждённый нож
3.2. Нож установлен с дисбалансом
3.3. Колёса на разных уровнях

Выберите.', false, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_corded_lawn_mower'), 'final_replace_sharpen_blade', 'Замена/заточка ножа: Заточить или заменить.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_corded_lawn_mower'), 'final_install_blade_correctly', 'Правильная установка ножа: Снять, проверить балансировку, переустановить.', true, NULL),
((SELECT id FROM fsm_scenarios WHERE name = 'diagnose_corded_lawn_mower'), 'final_adjust_height', 'Регулировка высоты кошения: Отрегулировать высоту.', true, NULL);
