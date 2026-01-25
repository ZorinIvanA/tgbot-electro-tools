-- Script to fix next_step_key values in fsm_steps table
-- This script analyzes the FSM structure and sets correct next_step values

-- First, let's create a temporary table to help with the analysis
CREATE TEMP TABLE step_analysis AS
SELECT 
    scenario_id,
    step_key,
    message,
    is_final,
    next_step_key,
    state_type,
    -- Determine the type of step based on content and naming
    CASE 
        WHEN step_key = 'root' THEN 'root'
        WHEN step_key LIKE '%_action' THEN 'action_selector'
        WHEN step_key LIKE '%_ok' OR step_key LIKE '%_not_ok' OR step_key LIKE '%_yes' OR step_key LIKE '%_no' THEN 'diagnostic_result'
        WHEN step_key LIKE '%_lit' OR step_key LIKE '%_dark' OR step_key LIKE '%_reacts' OR step_key LIKE '%_no_reaction' THEN 'diagnostic_question'
        WHEN is_final = true THEN 'final'
        ELSE 'intermediate'
    END as step_type
FROM fsm_steps;

-- Update next_step_key for root steps - they should point to the first problem step
UPDATE fsm_steps SET next_step_key = 'no_power' 
WHERE scenario_id = 1 AND step_key = 'root';

UPDATE fsm_steps SET next_step_key = 'no_power' 
WHERE scenario_id = 2 AND step_key = 'root';

UPDATE fsm_steps SET next_step_key = 'no_power' 
WHERE scenario_id = 3 AND step_key = 'root';

UPDATE fsm_steps SET next_step_key = 'no_power' 
WHERE scenario_id = 4 AND step_key = 'root';

UPDATE fsm_steps SET next_step_key = 'no_power' 
WHERE scenario_id = 5 AND step_key = 'root';

-- Update next_step_key for problem steps that have action selectors
UPDATE fsm_steps SET next_step_key = 'no_power_action' 
WHERE scenario_id = 1 AND step_key = 'no_power';

UPDATE fsm_steps SET next_step_key = 'no_power_action' 
WHERE scenario_id = 2 AND step_key = 'no_power';

UPDATE fsm_steps SET next_step_key = 'no_power_action' 
WHERE scenario_id = 3 AND step_key = 'no_power';

UPDATE fsm_steps SET next_step_key = 'no_power_action' 
WHERE scenario_id = 4 AND step_key = 'no_power';

UPDATE fsm_steps SET next_step_key = 'no_power_action' 
WHERE scenario_id = 5 AND step_key = 'no_power';

UPDATE fsm_steps SET next_step_key = 'stopped_action' 
WHERE scenario_id = 1 AND step_key = 'stops_during_work';

UPDATE fsm_steps SET next_step_key = 'motor_no_blade_action' 
WHERE scenario_id = 2 AND step_key = 'motor_runs_no_blade';

UPDATE fsm_steps SET next_step_key = 'vibration_action' 
WHERE scenario_id = 2 AND step_key = 'vibration_inaccurate';

UPDATE fsm_steps SET next_step_key = 'blade_not_moving_action' 
WHERE scenario_id = 3 AND step_key = 'blade_no_move';

UPDATE fsm_steps SET next_step_key = 'vibration_drift_action' 
WHERE scenario_id = 3 AND step_key = 'vibration_drift';

UPDATE fsm_steps SET next_step_key = 'spins_no_torque_action' 
WHERE scenario_id = 4 AND step_key = 'spins_no_torque';

UPDATE fsm_steps SET next_step_key = 'battery_drains_action' 
WHERE scenario_id = 4 AND step_key = 'battery_drains';

UPDATE fsm_steps SET next_step_key = 'motor_no_blade_action_lawnmower' 
WHERE scenario_id = 5 AND step_key = 'motor_runs_no_blade';

UPDATE fsm_steps SET next_step_key = 'uneven_cut_action' 
WHERE scenario_id = 5 AND step_key = 'uneven_vibration';

-- Update diagnostic question steps for angle grinder
UPDATE fsm_steps SET next_step_key = 'no_power_indicator_lit_reacts' 
WHERE scenario_id = 1 AND step_key = 'no_power_indicator_lit';

UPDATE fsm_steps SET next_step_key = 'no_power_indicator_lit_reacts_disk_turns' 
WHERE scenario_id = 1 AND step_key = 'no_power_indicator_lit_reacts';

UPDATE fsm_steps SET next_step_key = 'stops_after_time' 
WHERE scenario_id = 1 AND step_key = 'stops_immediately';

UPDATE fsm_steps SET next_step_key = 'stops_after_time_hot' 
WHERE scenario_id = 1 AND step_key = 'stops_after_time';

-- Update diagnostic question steps for miter saw
UPDATE fsm_steps SET next_step_key = 'no_power_power_ok' 
WHERE scenario_id = 2 AND step_key = 'no_power_no_power';

UPDATE fsm_steps SET next_step_key = 'no_power_power_ok_locks_ok' 
WHERE scenario_id = 2 AND step_key = 'no_power_power_ok';

UPDATE fsm_steps SET next_step_key = 'motor_runs_no_blade_belt_ok' 
WHERE scenario_id = 2 AND step_key = 'motor_runs_no_blade_belt_broken';

UPDATE fsm_steps SET next_step_key = 'vibration_inaccurate_disk_ok' 
WHERE scenario_id = 2 AND step_key = 'vibration_inaccurate_disk_problem';

-- Update diagnostic question steps for jigsaw
UPDATE fsm_steps SET next_step_key = 'no_power_power_ok' 
WHERE scenario_id = 3 AND step_key = 'no_power_no_power';

UPDATE fsm_steps SET next_step_key = 'blade_no_move_blade_ok' 
WHERE scenario_id = 3 AND step_key = 'blade_no_move_blade_not_ok';

UPDATE fsm_steps SET next_step_key = 'vibration_drift_blade_ok' 
WHERE scenario_id = 3 AND step_key = 'vibration_drift_blade_problem';

-- Update diagnostic question steps for cordless drill
UPDATE fsm_steps SET next_step_key = 'no_power_indicator_lit' 
WHERE scenario_id = 4 AND step_key = 'no_power_indicator_dark';

UPDATE fsm_steps SET next_step_key = 'spins_no_torque_clutch_ok' 
WHERE scenario_id = 4 AND step_key = 'spins_no_torque_clutch_triggered';

UPDATE fsm_steps SET next_step_key = 'battery_drains_old' 
WHERE scenario_id = 4 AND step_key = 'battery_drains_new';

-- Update diagnostic question steps for lawnmower
UPDATE fsm_steps SET next_step_key = 'no_power_power_ok' 
WHERE scenario_id = 5 AND step_key = 'no_power_no_power';

UPDATE fsm_steps SET next_step_key = 'motor_runs_no_blade_clear' 
WHERE scenario_id = 5 AND step_key = 'motor_runs_no_blade_blocked';

UPDATE fsm_steps SET next_step_key = 'uneven_vibration_wheels_ok' 
WHERE scenario_id = 5 AND step_key = 'uneven_vibration_wheels_not_level';

-- Update action selector steps to point to specific actions
-- Angle grinder actions
UPDATE fsm_steps SET next_step_key = 'charge_or_replace_battery' 
WHERE scenario_id = 1 AND step_key = 'no_power_action';

UPDATE fsm_steps SET next_step_key = 'restart_after_protection_reset' 
WHERE scenario_id = 1 AND step_key = 'stopped_action';

-- Miter saw actions
UPDATE fsm_steps SET next_step_key = 'restore_power' 
WHERE scenario_id = 2 AND step_key = 'no_power_action';

UPDATE fsm_steps SET next_step_key = 'repair_mechanical_drive_miter' 
WHERE scenario_id = 2 AND step_key = 'motor_no_blade_action';

UPDATE fsm_steps SET next_step_key = 'reinstall_blade' 
WHERE scenario_id = 2 AND step_key = 'vibration_action';

-- Jigsaw actions
UPDATE fsm_steps SET next_step_key = 'restore_power' 
WHERE scenario_id = 3 AND step_key = 'no_power_action';

UPDATE fsm_steps SET next_step_key = 'correctly_install_blade' 
WHERE scenario_id = 3 AND step_key = 'blade_not_moving_action';

UPDATE fsm_steps SET next_step_key = 'install_new_blade' 
WHERE scenario_id = 3 AND step_key = 'vibration_drift_action';

-- Cordless drill actions
UPDATE fsm_steps SET next_step_key = 'charge_or_replace_battery' 
WHERE scenario_id = 4 AND step_key = 'no_power_action';

UPDATE fsm_steps SET next_step_key = 'adjust_torque_setting' 
WHERE scenario_id = 4 AND step_key = 'spins_no_torque_action';

UPDATE fsm_steps SET next_step_key = 'replace_battery_cells' 
WHERE scenario_id = 4 AND step_key = 'battery_drains_action';

-- Lawnmower actions
UPDATE fsm_steps SET next_step_key = 'restore_power' 
WHERE scenario_id = 5 AND step_key = 'no_power_action';

UPDATE fsm_steps SET next_step_key = 'clear_working_area' 
WHERE scenario_id = 5 AND step_key = 'motor_no_blade_action_lawnmower';

UPDATE fsm_steps SET next_step_key = 'sharpen_or_replace_blade' 
WHERE scenario_id = 5 AND step_key = 'uneven_cut_action';

-- Verify the results
SELECT 
    scenario_id,
    step_key,
    next_step_key,
    is_final,
    state_type
FROM fsm_steps 
WHERE next_step_key IS NOT NULL
ORDER BY scenario_id, step_key;

-- Show steps that still have NULL next_step_key (should be final steps)
SELECT 
    scenario_id,
    step_key,
    next_step_key,
    is_final,
    state_type
FROM fsm_steps 
WHERE next_step_key IS NULL
ORDER BY scenario_id, step_key;

-- Clean up temporary table
DROP TABLE step_analysis;
