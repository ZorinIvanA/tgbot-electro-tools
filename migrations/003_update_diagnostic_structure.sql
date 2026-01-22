-- 003_update_diagnostic_structure.sql
-- Update diagnostic structure with multi-level interactive diagnostics

-- Add display_name column to scenarios if not exists
ALTER TABLE fsm_scenarios ADD COLUMN IF NOT EXISTS display_name TEXT;

-- Clear existing data to rebuild with new interactive structure
TRUNCATE TABLE user_sessions CASCADE;
TRUNCATE TABLE fsm_steps CASCADE;
TRUNCATE TABLE fsm_scenarios CASCADE;

-- Reset sequences
ALTER SEQUENCE fsm_scenarios_id_seq RESTART WITH 1;
ALTER SEQUENCE fsm_steps_id_seq RESTART WITH 1;

-- Insert updated scenarios with Russian display names
INSERT INTO fsm_scenarios (name, display_name, trigger_keywords, description) VALUES
('diagnose_angle_grinder', 'Угловая шлифовальная машина', ARRAY['угловая шлифовальная машина', 'болгарка', 'ушм', 'angle grinder'], 'Диагностика угловой шлифовальной машины'),
('diagnose_miter_saw', 'Торцовочная пила', ARRAY['торцовочная пила', 'торцовка', 'miter saw'], 'Диагностика торцовочной пилы'),
('diagnose_jigsaw', 'Электролобзик', ARRAY['электролобзик', 'лобзик', 'jigsaw'], 'Диагностика электролобзика'),
('diagnose_cordless_drill', 'Аккумуляторный шуруповёрт', ARRAY['аккумуляторный шуруповёрт', 'шуруповёрт', 'cordless screwdriver'], 'Диагностика аккумуляторного шуруповёрта'),
('diagnose_corded_lawnmower', 'Проводная (сетевая) газонокосилка', ARRAY['проводная газонокосилка', 'газонокосилка', 'lawn mower'], 'Диагностика проводной газонокосилки');

-- Insert interactive diagnostic steps for angle grinder
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
-- Root state: problem selection
(1, 'root', 'Диагностика угловой шлифовальной машины. Выберите проблему:', false, NULL),

-- Problem: not turning on
(1, 'no_power', 'Устройство не включается.

Проверьте индикатор заряда аккумулятора. Горит ли он?', false, NULL),

-- Diagnostic branch: indicator check
(1, 'no_power_indicator_lit', 'Индикатор горит. Теперь проверьте кнопку пуска.

Нажмите кнопку пуска. Есть ли какая-либо реакция на нажатие (звук, вибрация)?', false, NULL),

(1, 'no_power_indicator_lit_reacts', 'Есть реакция на кнопку. Проверьте предохранитель шпинделя.

Поворачивается ли диск вручную при отжатой защите шпинделя?', false, NULL),

(1, 'no_power_indicator_lit_reacts_disk_turns', 'Диск вращается. Возможно, сработала защита от перегрева.

Дайте устройству остыть 15-30 минут и попробуйте снова.', true, NULL),

(1, 'no_power_indicator_lit_reacts_disk_stuck', 'Диск не вращается. Возможно, механическое заклинивание.

Разберите и осмотрите редуктор. Замените повреждённые шестерни.', true, NULL),

(1, 'no_power_indicator_lit_no_reaction', 'Нет реакции на кнопку. Проверьте кнопку пуска.

Разберите и осмотрите кнопку. Зачистите контакты или замените кнопку.', true, NULL),

(1, 'no_power_indicator_dark', 'Индикатор не горит. Аккумулятор разряжен.

Подключите зарядное устройство и зарядите аккумулятор полностью (1-2 часа).', true, NULL),

-- Problem: stops during operation
(1, 'stops_during_work', 'Устройство останавливается во время работы.

Сколько времени работает до остановки? Останавливается сразу или через некоторое время?', false, NULL),

(1, 'stops_immediately', 'Останавливается сразу. Проверьте соединение аккумулятора.

Снимите и установите аккумулятор заново. Проверьте контакты на окисление.', true, NULL),

(1, 'stops_after_time', 'Останавливается через время. Возможно, перегрев или разряд.

Во время работы чувствуете ли вы нагрев корпуса?', false, NULL),

(1, 'stops_after_time_hot', 'Корпус горячий. Сработала термозащита.

Дайте остыть 15-30 минут. Работайте с перерывами, не перегружайте.', true, NULL),

(1, 'stops_after_time_not_hot', 'Корпус не сильно нагревается. Проверьте аккумулятор.

Аккумулятор может быть неисправен. Замените на заряженный.', true, NULL),

-- Problem: vibration or unusual noise
(1, 'vibration_noise', 'Вибрация или необычный шум.

Вибрация сильная или лёгкая? Шум скрежет или гул?', false, NULL),

(1, 'strong_vibration', 'Сильная вибрация. Осмотрите диск и зажим.

Проверьте правильность установки диска. Подтяните зажимную гайку.', true, NULL),

(1, 'grinding_noise', 'Скрежет. Возможно, износ щёток или подшипников.

Разберите и осмотрите щётки. Замените изношенные (менее 5мм).', true, NULL),

(1, 'other_noise', 'Другой шум. Проверьте редуктор.

Осмотрите шестерни редуктора. Замените повреждённые детали.', true, NULL);

-- Insert interactive diagnostic steps for miter saw
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
-- Root state
(2, 'root', 'Диагностика торцовочной пилы. Выберите проблему:', false, NULL),

-- Problem: not turning on
(2, 'no_power', 'Устройство не включается.

Проверьте подключение к сети. Розетка работает? Шнур повреждён?', false, NULL),

(2, 'no_power_power_ok', 'Питание в порядке. Проверьте блокировку пуска.

Защитный кожух опущен? Ручка зафиксирована? Кнопка блокировки отжата?', false, NULL),

(2, 'no_power_power_ok_locks_ok', 'Блокировки в порядке. Проверьте предохранитель.

Нажмите кнопку сброса предохранителя (обычно красная кнопка).', true, NULL),

(2, 'no_power_power_ok_locks_not_ok', 'Проверьте блокировки. Убедитесь, что:

• Защитный кожух полностью опущен
• Рукоятка правильно зафиксирована
• Нет активных блокировок безопасности', true, NULL),

(2, 'no_power_no_power', 'Проблема с питанием.

Проверьте розетку другим устройством. Осмотрите шнур на повреждения.', true, NULL),

-- Problem: motor runs but blade doesn't spin
(2, 'motor_runs_no_blade', 'Мотор работает, но диск не вращается.

Проверьте ремень привода. Цел ли он? Правильно ли натянут?', false, NULL),

(2, 'motor_runs_no_blade_belt_ok', 'Ремень в порядке. Проверьте шпиндель.

Осмотрите шпиндель на заклинивание. Проверьте подшипники.', true, NULL),

(2, 'motor_runs_no_blade_belt_broken', 'Ремень повреждён.

Замените ремень привода. Убедитесь в правильном размере и установке.', true, NULL),

-- Problem: vibration or inaccurate cut
(2, 'vibration_inaccurate', 'Вибрация или неточный рез.

Диск правильно установлен? Гайка затянута? Диск целый?', false, NULL),

(2, 'vibration_inaccurate_disk_ok', 'Диск в порядке. Проверьте параллельность.

Осмотрите направляющие. Отрегулируйте положение диска.', true, NULL),

(2, 'vibration_inaccurate_disk_problem', 'Проблема с диском.

Переустановите диск правильно. Замените повреждённый диск.', true, NULL);

-- Insert interactive diagnostic steps for jigsaw
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
-- Root state
(3, 'root', 'Диагностика электролобзика. Выберите проблему:', false, NULL),

-- Problem: not turning on
(3, 'no_power', 'Устройство не включается.

Проверьте подключение к сети. Розетка работает?', false, NULL),

(3, 'no_power_power_ok', 'Питание в порядке. Проверьте кнопку пуска.

Разберите и осмотрите кнопку. Зачистите контакты от пыли.', true, NULL),

(3, 'no_power_no_power', 'Проблема с питанием.

Проверьте розетку. Осмотрите кабель на повреждения.', true, NULL),

-- Problem: blade doesn't move
(3, 'blade_no_move', 'Полотно не движется.

Полото правильно установлено? Зажим зафиксирован?', false, NULL),

(3, 'blade_no_move_blade_ok', 'Полотно установлено правильно. Проверьте механизм.

Осмотрите кривошип и шток. Замените повреждённые детали.', true, NULL),

(3, 'blade_no_move_blade_not_ok', 'Проблема с установкой полотна.

Осмотрите зажим. Правильно установите полотно подходящего типа.', true, NULL),

-- Problem: vibration or drift
(3, 'vibration_drift', 'Вибрация или увод в сторону.

Полотно подходящее? Ролики в порядке?', false, NULL),

(3, 'vibration_drift_blade_ok', 'Полотно в порядке. Проверьте ролики.

Замените изношенные направляющие ролики.', true, NULL),

(3, 'vibration_drift_blade_problem', 'Полотно неподходящее.

Выберите полотно по материалу и толщине. Используйте качественное полотно.', true, NULL);

-- Insert interactive diagnostic steps for cordless drill
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
-- Root state
(4, 'root', 'Диагностика аккумуляторного шуруповёрта. Выберите проблему:', false, NULL),

-- Problem: not turning on
(4, 'no_power', 'Устройство не включается.

Проверьте индикатор аккумулятора. Горит ли он?', false, NULL),

(4, 'no_power_indicator_lit', 'Индикатор горит. Проверьте кнопку.

Разберите и зачистите контакты кнопки реверса.', true, NULL),

(4, 'no_power_indicator_dark', 'Индикатор не горит.

Зарядите аккумулятор или замените на заряженный.', true, NULL),

-- Problem: spins but no torque
(4, 'spins_no_torque', 'Вращается, но не крутит.

Муфта момента сработала? Кольцо стоит на высокой цифре?', false, NULL),

(4, 'spins_no_torque_clutch_ok', 'Муфта в порядке. Проверьте редуктор.

Разберите редуктор. Замените изношенные шестерни.', true, NULL),

(4, 'spins_no_torque_clutch_triggered', 'Муфта сработала.

Увеличьте настройку момента на кольце регулятора.', true, NULL),

-- Problem: battery drains quickly
(4, 'battery_drains', 'Аккумулятор быстро садится.

Аккумулятор старый? Долго использовался?', false, NULL),

(4, 'battery_drains_old', 'Аккумулятор изношен.

Замените аккумулятор на новый оригинальный.', true, NULL),

(4, 'battery_drains_new', 'Аккумулятор новый. Проверьте плату BMS.

Возможно, неисправность системы управления. Обратитесь в сервис.', true, NULL);

-- Insert interactive diagnostic steps for corded lawnmower
INSERT INTO fsm_steps (scenario_id, step_key, message, is_final, next_step_key) VALUES
-- Root state
(5, 'root', 'Диагностика проводной газонокосилки. Выберите проблему:', false, NULL),

-- Problem: not turning on
(5, 'no_power', 'Устройство не включается.

Проверьте подключение к сети. Розетка работает?', false, NULL),

(5, 'no_power_power_ok', 'Питание в порядке. Проверьте термозащиту.

Дайте остыть 20-30 минут. Нажмите кнопку сброса предохранителя.', true, NULL),

(5, 'no_power_no_power', 'Проблема с питанием.

Проверьте розетку и кабель. Замените повреждённый кабель.', true, NULL),

-- Problem: motor runs but blade doesn't spin
(5, 'motor_runs_no_blade', 'Мотор работает, но нож не вращается.

Осмотрите нож. Нет ли посторонних предметов?', false, NULL),

(5, 'motor_runs_no_blade_clear', 'Нож чистый. Проверьте ремень.

Осмотрите ремень привода. Замените повреждённый ремень.', true, NULL),

(5, 'motor_runs_no_blade_blocked', 'Нож заблокирован.

Очистите нож и защитный кожух от травы и посторонних предметов.', true, NULL),

-- Problem: uneven cut or vibration
(5, 'uneven_vibration', 'Неровный срез или вибрация.

Колёса на одинаковой высоте? Нож целый?', false, NULL),

(5, 'uneven_vibration_wheels_ok', 'Колёса в порядке. Проверьте нож.

Заточите или замените тупой/повреждённый нож.', true, NULL),

(5, 'uneven_vibration_wheels_not_level', 'Колёса на разной высоте.

Отрегулируйте высоту всех колёс одинаково.', true, NULL);
