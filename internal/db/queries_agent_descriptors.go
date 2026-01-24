package db

import (
	"database/sql"
	"encoding/json"
)

// GetAgentDescriptors returns all agent descriptors stored in SQLite.
func GetAgentDescriptors(db *sql.DB) ([]AgentDescriptor, error) {
	rows, err := db.Query(`
		SELECT agent_id, display_name, capabilities, updated_at
		FROM fray_agent_descriptors
		ORDER BY agent_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var descriptors []AgentDescriptor
	for rows.Next() {
		var agentID string
		var displayName sql.NullString
		var capabilities sql.NullString
		var updatedAt sql.NullInt64
		if err := rows.Scan(&agentID, &displayName, &capabilities, &updatedAt); err != nil {
			return nil, err
		}

		var caps []string
		if capabilities.Valid && capabilities.String != "" {
			if err := json.Unmarshal([]byte(capabilities.String), &caps); err != nil {
				return nil, err
			}
		}

		var name *string
		if displayName.Valid {
			value := displayName.String
			name = &value
		}

		descriptor := AgentDescriptor{
			AgentID:      agentID,
			DisplayName:  name,
			Capabilities: caps,
		}
		if updatedAt.Valid {
			descriptor.TS = updatedAt.Int64
		}
		descriptors = append(descriptors, descriptor)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return descriptors, nil
}

// GetAgentDescriptor returns a descriptor for the given agent ID.
func GetAgentDescriptor(db *sql.DB, agentID string) (*AgentDescriptor, error) {
	row := db.QueryRow(`
		SELECT agent_id, display_name, capabilities, updated_at
		FROM fray_agent_descriptors
		WHERE agent_id = ?
	`, agentID)

	var id string
	var displayName sql.NullString
	var capabilities sql.NullString
	var updatedAt sql.NullInt64
	if err := row.Scan(&id, &displayName, &capabilities, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	var caps []string
	if capabilities.Valid && capabilities.String != "" {
		if err := json.Unmarshal([]byte(capabilities.String), &caps); err != nil {
			return nil, err
		}
	}

	var name *string
	if displayName.Valid {
		value := displayName.String
		name = &value
	}

	descriptor := &AgentDescriptor{
		AgentID:      id,
		DisplayName:  name,
		Capabilities: caps,
	}
	if updatedAt.Valid {
		descriptor.TS = updatedAt.Int64
	}
	return descriptor, nil
}
