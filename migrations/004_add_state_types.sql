-- 004_add_state_types.sql
-- Add state_type field to fsm_steps table to categorize states as start, intermediate, or final

-- Add state_type column
ALTER TABLE fsm_steps ADD COLUMN state_type TEXT NOT NULL DEFAULT 'intermediate' CHECK (state_type IN ('start', 'intermediate', 'final'));

-- Update existing steps with appropriate types
-- Start states: root steps for problem selection
UPDATE fsm_steps SET state_type = 'start' WHERE step_key = 'root';

-- Intermediate states: diagnostic questions and branches
UPDATE fsm_steps SET state_type = 'intermediate' WHERE step_key LIKE '%no_power%' AND step_key != 'no_power';
UPDATE fsm_steps SET state_type = 'intermediate' WHERE step_key LIKE '%stops%' AND step_key != 'stops_during_work';
UPDATE fsm_steps SET state_type = 'intermediate' WHERE step_key LIKE '%vibration%' AND step_key != 'vibration_noise';
UPDATE fsm_steps SET state_type = 'intermediate' WHERE step_key LIKE '%motor_runs%' AND step_key != 'motor_runs_no_blade';
UPDATE fsm_steps SET state_type = 'intermediate' WHERE step_key LIKE '%blade%' AND step_key NOT IN ('blade_no_move', 'no_power');
UPDATE fsm_steps SET state_type = 'intermediate' WHERE step_key LIKE '%indicator%' OR step_key LIKE '%button%' OR step_key LIKE '%disk%' OR step_key LIKE '%belt%' OR step_key LIKE '%clutch%' OR step_key LIKE '%old%' OR step_key LIKE '%new%' OR step_key LIKE '%clear%' OR step_key LIKE '%blocked%' OR step_key LIKE '%wheels%';

-- Final states: action/recommendation steps
UPDATE fsm_steps SET state_type = 'final' WHERE is_final = true;

-- Override specific cases that should be intermediate
UPDATE fsm_steps SET state_type = 'intermediate' WHERE step_key IN ('no_power', 'stops_during_work', 'vibration_noise', 'motor_runs_no_blade', 'blade_no_move', 'spins_no_torque', 'battery_drains', 'uneven_vibration');
