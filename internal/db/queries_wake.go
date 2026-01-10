package db

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/types"
)

// CreateWakeCondition creates a new wake condition in the database and JSONL.
func CreateWakeCondition(db *sql.DB, projectPath string, input types.WakeConditionInput) (*types.WakeCondition, error) {
	guid, err := core.GenerateGUID("wake")
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	condition := &types.WakeCondition{
		GUID:           guid,
		AgentID:        input.AgentID,
		SetBy:          input.SetBy,
		Type:           input.Type,
		Pattern:        input.Pattern,
		OnAgents:       input.OnAgents,
		InThread:       input.InThread,
		AfterMs:        input.AfterMs,
		UseRouter:      input.UseRouter,
		Prompt:         input.Prompt,
		PromptText:     input.PromptText,
		PollIntervalMs: input.PollIntervalMs,
		PersistMode:    input.PersistMode,
		Paused:         false,
		CreatedAt:      now,
	}

	// Calculate expiry for "after" conditions
	if input.Type == types.WakeConditionAfter && input.AfterMs != nil {
		expiresAt := now + (*input.AfterMs / 1000)
		condition.ExpiresAt = &expiresAt
	}

	// Marshal JSON fields
	onAgentsJSON, err := json.Marshal(input.OnAgents)
	if err != nil {
		return nil, err
	}

	// Insert into database
	_, err = db.Exec(`
		INSERT INTO fray_wake_conditions (
			guid, agent_id, set_by, type, pattern, on_agents, in_thread,
			after_ms, use_router, prompt, prompt_text, poll_interval_ms, persist_mode, paused, created_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		condition.GUID,
		condition.AgentID,
		condition.SetBy,
		string(condition.Type),
		condition.Pattern,
		string(onAgentsJSON),
		condition.InThread,
		condition.AfterMs,
		condition.UseRouter,
		condition.Prompt,
		condition.PromptText,
		condition.PollIntervalMs,
		string(condition.PersistMode),
		condition.Paused,
		condition.CreatedAt,
		condition.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}

	// Append to JSONL
	if err := AppendWakeCondition(projectPath, *condition); err != nil {
		return nil, err
	}

	return condition, nil
}

// GetWakeConditions retrieves wake conditions, optionally filtered by agent.
// By default excludes paused conditions; use includePaused=true to include them.
func GetWakeConditions(db *sql.DB, agentID string) ([]types.WakeCondition, error) {
	return GetWakeConditionsFiltered(db, agentID, false)
}

// GetWakeConditionsFiltered retrieves wake conditions with optional paused filter.
func GetWakeConditionsFiltered(db *sql.DB, agentID string, includePaused bool) ([]types.WakeCondition, error) {
	query := `
		SELECT guid, agent_id, set_by, type, pattern, on_agents, in_thread,
		       after_ms, use_router, prompt, prompt_text, poll_interval_ms, last_polled_at,
		       persist_mode, paused, created_at, expires_at
		FROM fray_wake_conditions
		WHERE (expires_at IS NULL OR expires_at > ?)
	`
	args := []any{time.Now().Unix()}

	if !includePaused {
		query += " AND paused = 0"
	}

	if agentID != "" {
		query += " AND agent_id = ?"
		args = append(args, agentID)
	}

	query += " ORDER BY created_at ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conditions []types.WakeCondition
	for rows.Next() {
		var c types.WakeCondition
		var condType string
		var onAgentsJSON string
		var pattern, inThread, prompt, promptText, persistMode sql.NullString
		var afterMs, pollIntervalMs, lastPolledAt, expiresAt sql.NullInt64

		err := rows.Scan(
			&c.GUID,
			&c.AgentID,
			&c.SetBy,
			&condType,
			&pattern,
			&onAgentsJSON,
			&inThread,
			&afterMs,
			&c.UseRouter,
			&prompt,
			&promptText,
			&pollIntervalMs,
			&lastPolledAt,
			&persistMode,
			&c.Paused,
			&c.CreatedAt,
			&expiresAt,
		)
		if err != nil {
			return nil, err
		}

		c.Type = types.WakeConditionType(condType)

		if pattern.Valid {
			c.Pattern = &pattern.String
		}
		if inThread.Valid {
			c.InThread = &inThread.String
		}
		if afterMs.Valid {
			c.AfterMs = &afterMs.Int64
		}
		if prompt.Valid {
			c.Prompt = &prompt.String
		}
		if promptText.Valid {
			c.PromptText = &promptText.String
		}
		if pollIntervalMs.Valid {
			c.PollIntervalMs = &pollIntervalMs.Int64
		}
		if lastPolledAt.Valid {
			c.LastPolledAt = &lastPolledAt.Int64
		}
		if persistMode.Valid {
			c.PersistMode = types.WakePersistMode(persistMode.String)
		}
		if expiresAt.Valid {
			c.ExpiresAt = &expiresAt.Int64
		}

		if onAgentsJSON != "" && onAgentsJSON != "null" {
			if err := json.Unmarshal([]byte(onAgentsJSON), &c.OnAgents); err != nil {
				c.OnAgents = nil
			}
		}

		conditions = append(conditions, c)
	}

	return conditions, rows.Err()
}

// GetWakeCondition retrieves a single wake condition by GUID.
func GetWakeCondition(db *sql.DB, guid string) (*types.WakeCondition, error) {
	var c types.WakeCondition
	var condType string
	var onAgentsJSON string
	var pattern, inThread, prompt, promptText, persistMode sql.NullString
	var afterMs, pollIntervalMs, lastPolledAt, expiresAt sql.NullInt64

	err := db.QueryRow(`
		SELECT guid, agent_id, set_by, type, pattern, on_agents, in_thread,
		       after_ms, use_router, prompt, prompt_text, poll_interval_ms, last_polled_at,
		       persist_mode, paused, created_at, expires_at
		FROM fray_wake_conditions
		WHERE guid = ?
	`, guid).Scan(
		&c.GUID,
		&c.AgentID,
		&c.SetBy,
		&condType,
		&pattern,
		&onAgentsJSON,
		&inThread,
		&afterMs,
		&c.UseRouter,
		&prompt,
		&promptText,
		&pollIntervalMs,
		&lastPolledAt,
		&persistMode,
		&c.Paused,
		&c.CreatedAt,
		&expiresAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	c.Type = types.WakeConditionType(condType)

	if pattern.Valid {
		c.Pattern = &pattern.String
	}
	if inThread.Valid {
		c.InThread = &inThread.String
	}
	if afterMs.Valid {
		c.AfterMs = &afterMs.Int64
	}
	if prompt.Valid {
		c.Prompt = &prompt.String
	}
	if promptText.Valid {
		c.PromptText = &promptText.String
	}
	if pollIntervalMs.Valid {
		c.PollIntervalMs = &pollIntervalMs.Int64
	}
	if lastPolledAt.Valid {
		c.LastPolledAt = &lastPolledAt.Int64
	}
	if persistMode.Valid {
		c.PersistMode = types.WakePersistMode(persistMode.String)
	}
	if expiresAt.Valid {
		c.ExpiresAt = &expiresAt.Int64
	}

	if onAgentsJSON != "" && onAgentsJSON != "null" {
		if err := json.Unmarshal([]byte(onAgentsJSON), &c.OnAgents); err != nil {
			c.OnAgents = nil
		}
	}

	return &c, nil
}

// ClearWakeConditions removes all wake conditions for an agent.
func ClearWakeConditions(db *sql.DB, projectPath string, agentID string) (int64, error) {
	result, err := db.Exec(`DELETE FROM fray_wake_conditions WHERE agent_id = ?`, agentID)
	if err != nil {
		return 0, err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	// Append clear record to JSONL
	if count > 0 {
		if err := AppendWakeConditionClear(projectPath, agentID); err != nil {
			return count, err
		}
	}

	return count, nil
}

// DeleteWakeCondition removes a specific wake condition.
func DeleteWakeCondition(db *sql.DB, projectPath string, guid string) error {
	_, err := db.Exec(`DELETE FROM fray_wake_conditions WHERE guid = ?`, guid)
	if err != nil {
		return err
	}

	// Append delete record to JSONL
	return AppendWakeConditionDelete(projectPath, guid)
}

// GetPatternWakeConditions retrieves all pattern-based wake conditions that are active.
func GetPatternWakeConditions(db *sql.DB) ([]types.WakeCondition, error) {
	rows, err := db.Query(`
		SELECT guid, agent_id, set_by, type, pattern, on_agents, in_thread,
		       after_ms, use_router, prompt, persist_mode, paused, created_at, expires_at
		FROM fray_wake_conditions
		WHERE type = ?
		  AND paused = 0
		  AND (expires_at IS NULL OR expires_at > ?)
	`, string(types.WakeConditionPattern), time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conditions []types.WakeCondition
	for rows.Next() {
		var c types.WakeCondition
		var condType string
		var onAgentsJSON string
		var pattern, inThread, prompt, persistMode sql.NullString
		var afterMs, expiresAt sql.NullInt64

		err := rows.Scan(
			&c.GUID,
			&c.AgentID,
			&c.SetBy,
			&condType,
			&pattern,
			&onAgentsJSON,
			&inThread,
			&afterMs,
			&c.UseRouter,
			&prompt,
			&persistMode,
			&c.Paused,
			&c.CreatedAt,
			&expiresAt,
		)
		if err != nil {
			return nil, err
		}

		c.Type = types.WakeConditionType(condType)

		if pattern.Valid {
			c.Pattern = &pattern.String
		}
		if inThread.Valid {
			c.InThread = &inThread.String
		}
		if afterMs.Valid {
			c.AfterMs = &afterMs.Int64
		}
		if prompt.Valid {
			c.Prompt = &prompt.String
		}
		if persistMode.Valid {
			c.PersistMode = types.WakePersistMode(persistMode.String)
		}
		if expiresAt.Valid {
			c.ExpiresAt = &expiresAt.Int64
		}

		if onAgentsJSON != "" && onAgentsJSON != "null" {
			if err := json.Unmarshal([]byte(onAgentsJSON), &c.OnAgents); err != nil {
				c.OnAgents = nil
			}
		}

		conditions = append(conditions, c)
	}

	return conditions, rows.Err()
}

// GetExpiredWakeConditions retrieves wake conditions that have expired.
func GetExpiredWakeConditions(db *sql.DB) ([]types.WakeCondition, error) {
	rows, err := db.Query(`
		SELECT guid, agent_id, set_by, type, pattern, on_agents, in_thread,
		       after_ms, use_router, prompt, persist_mode, paused, created_at, expires_at
		FROM fray_wake_conditions
		WHERE expires_at IS NOT NULL AND expires_at <= ?
	`, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conditions []types.WakeCondition
	for rows.Next() {
		var c types.WakeCondition
		var condType string
		var onAgentsJSON string
		var pattern, inThread, prompt, persistMode sql.NullString
		var afterMs, expiresAt sql.NullInt64

		err := rows.Scan(
			&c.GUID,
			&c.AgentID,
			&c.SetBy,
			&condType,
			&pattern,
			&onAgentsJSON,
			&inThread,
			&afterMs,
			&c.UseRouter,
			&prompt,
			&persistMode,
			&c.Paused,
			&c.CreatedAt,
			&expiresAt,
		)
		if err != nil {
			return nil, err
		}

		c.Type = types.WakeConditionType(condType)

		if pattern.Valid {
			c.Pattern = &pattern.String
		}
		if inThread.Valid {
			c.InThread = &inThread.String
		}
		if afterMs.Valid {
			c.AfterMs = &afterMs.Int64
		}
		if prompt.Valid {
			c.Prompt = &prompt.String
		}
		if persistMode.Valid {
			c.PersistMode = types.WakePersistMode(persistMode.String)
		}
		if expiresAt.Valid {
			c.ExpiresAt = &expiresAt.Int64
		}

		if onAgentsJSON != "" && onAgentsJSON != "null" {
			if err := json.Unmarshal([]byte(onAgentsJSON), &c.OnAgents); err != nil {
				c.OnAgents = nil
			}
		}

		conditions = append(conditions, c)
	}

	return conditions, rows.Err()
}

// PruneExpiredWakeConditions removes expired wake conditions.
func PruneExpiredWakeConditions(db *sql.DB) (int64, error) {
	result, err := db.Exec(`
		DELETE FROM fray_wake_conditions
		WHERE expires_at IS NOT NULL AND expires_at <= ?
	`, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// PauseWakeConditions pauses all persist-restore-on-back conditions for an agent.
// Called when agent lands (bye).
func PauseWakeConditions(db *sql.DB, projectPath string, agentID string) (int64, error) {
	result, err := db.Exec(`
		UPDATE fray_wake_conditions
		SET paused = 1
		WHERE agent_id = ? AND persist_mode = ?
	`, agentID, string(types.WakePersistRestoreOnBack))
	if err != nil {
		return 0, err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	if count > 0 {
		if err := AppendWakeConditionPause(projectPath, agentID); err != nil {
			return count, err
		}
	}

	return count, nil
}

// ResumeWakeConditions resumes all paused conditions for an agent.
// Called when agent returns (back).
func ResumeWakeConditions(db *sql.DB, projectPath string, agentID string) (int64, error) {
	result, err := db.Exec(`
		UPDATE fray_wake_conditions
		SET paused = 0
		WHERE agent_id = ? AND paused = 1
	`, agentID)
	if err != nil {
		return 0, err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	if count > 0 {
		if err := AppendWakeConditionResume(projectPath, agentID); err != nil {
			return count, err
		}
	}

	return count, nil
}

// ClearPersistUntilByeConditions removes all persist-until-bye conditions for an agent.
// Called when agent lands (bye).
func ClearPersistUntilByeConditions(db *sql.DB, projectPath string, agentID string) (int64, error) {
	result, err := db.Exec(`
		DELETE FROM fray_wake_conditions
		WHERE agent_id = ? AND persist_mode = ?
	`, agentID, string(types.WakePersistUntilBye))
	if err != nil {
		return 0, err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	if count > 0 {
		if err := AppendWakeConditionClearByBye(projectPath, agentID); err != nil {
			return count, err
		}
	}

	return count, nil
}

// ResetTimerCondition resets a timer-based wake condition for re-triggering.
// Called when a persistent timer condition fires.
func ResetTimerCondition(db *sql.DB, projectPath string, guid string, newExpiresAt int64) error {
	_, err := db.Exec(`
		UPDATE fray_wake_conditions
		SET expires_at = ?, created_at = ?
		WHERE guid = ?
	`, newExpiresAt, time.Now().Unix(), guid)
	if err != nil {
		return err
	}

	return AppendWakeConditionReset(projectPath, guid, newExpiresAt)
}

// UpdateLastPolledAt updates the last_polled_at timestamp for a prompt condition.
func UpdateLastPolledAt(db *sql.DB, guid string, polledAt int64) error {
	_, err := db.Exec(`
		UPDATE fray_wake_conditions
		SET last_polled_at = ?
		WHERE guid = ?
	`, polledAt, guid)
	return err
}
