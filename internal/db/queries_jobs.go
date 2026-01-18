package db

import (
	"database/sql"
	"encoding/json"

	"github.com/adamavenir/fray/internal/types"
)

// CreateJob inserts a new job.
func CreateJob(db *sql.DB, job types.Job) error {
	guid := job.GUID
	if guid == "" {
		var err error
		guid, err = generateUniqueGUIDForTable(db, "fray_jobs", "job")
		if err != nil {
			return err
		}
	}

	var contextJSON *string
	if job.Context != nil {
		data, err := json.Marshal(job.Context)
		if err != nil {
			return err
		}
		s := string(data)
		contextJSON = &s
	}

	_, err := db.Exec(`
		INSERT INTO fray_jobs (guid, name, context, owner_agent, status, thread_guid, created_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, guid, job.Name, contextJSON, job.OwnerAgent, string(job.Status), job.ThreadGUID, job.CreatedAt, job.CompletedAt)
	return err
}

// GetJob returns a job by GUID.
func GetJob(db *sql.DB, guid string) (*types.Job, error) {
	row := db.QueryRow(`
		SELECT guid, name, context, owner_agent, status, thread_guid, created_at, completed_at
		FROM fray_jobs
		WHERE guid = ?
	`, guid)

	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// UpdateJobStatus updates job status and optionally completed_at.
func UpdateJobStatus(db *sql.DB, guid string, status types.JobStatus, completedAt *int64) error {
	_, err := db.Exec(`
		UPDATE fray_jobs SET status = ?, completed_at = ? WHERE guid = ?
	`, string(status), completedAt, guid)
	return err
}

// GetJobWorkers returns all agents that are workers for a given job.
func GetJobWorkers(db *sql.DB, jobGuid string) ([]types.Agent, error) {
	rows, err := db.Query(`
		SELECT guid, agent_id, aap_guid, status, purpose, avatar, registered_at, last_seen, left_at, managed, invoke, presence, presence_changed_at, mention_watermark, reaction_watermark, last_heartbeat, last_session_id, session_mode, job_id, job_idx, is_ephemeral, last_known_input, last_known_output, tokens_updated_at
		FROM fray_agents
		WHERE job_id = ?
		ORDER BY job_idx
	`, jobGuid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []types.Agent
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return agents, nil
}

// GetActiveJobs returns all jobs with status 'running'.
func GetActiveJobs(db *sql.DB) ([]types.Job, error) {
	rows, err := db.Query(`
		SELECT guid, name, context, owner_agent, status, thread_guid, created_at, completed_at
		FROM fray_jobs
		WHERE status = 'running'
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []types.Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

// GetAllJobs returns all jobs regardless of status.
func GetAllJobs(db *sql.DB) ([]types.Job, error) {
	rows, err := db.Query(`
		SELECT guid, name, context, owner_agent, status, thread_guid, created_at, completed_at
		FROM fray_jobs
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []types.Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

// IsAmbiguousMention returns true if the base agent has active job workers,
// meaning a bare @agent mention would be ambiguous.
// Worker IDs use bracket notation: dev[abc-1], not dot notation.
func IsAmbiguousMention(db *sql.DB, baseAgent string) (bool, error) {
	// Check if there are any ephemeral agents with this base whose job is still running
	// Worker format: baseAgent[jobid-idx] e.g. dev[abc-1]
	row := db.QueryRow(`
		SELECT 1 FROM fray_agents a
		JOIN fray_jobs j ON a.job_id = j.guid
		WHERE a.is_ephemeral = 1
		  AND j.status = 'running'
		  AND (a.agent_id = ? OR a.agent_id LIKE ?)
		LIMIT 1
	`, baseAgent, baseAgent+"[%")

	var exists int
	err := row.Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func scanJob(scanner interface{ Scan(dest ...any) error }) (types.Job, error) {
	var row jobRow
	if err := scanner.Scan(&row.GUID, &row.Name, &row.Context, &row.OwnerAgent, &row.Status, &row.ThreadGUID, &row.CreatedAt, &row.CompletedAt); err != nil {
		return types.Job{}, err
	}
	return row.toJob(), nil
}

type jobRow struct {
	GUID        string
	Name        string
	Context     sql.NullString
	OwnerAgent  sql.NullString
	Status      string
	ThreadGUID  sql.NullString
	CreatedAt   int64
	CompletedAt sql.NullInt64
}

func (row jobRow) toJob() types.Job {
	job := types.Job{
		GUID:        row.GUID,
		Name:        row.Name,
		Status:      types.JobStatus(row.Status),
		CreatedAt:   row.CreatedAt,
		CompletedAt: nullIntPtr(row.CompletedAt),
	}
	if row.OwnerAgent.Valid {
		job.OwnerAgent = row.OwnerAgent.String
	}
	if row.ThreadGUID.Valid {
		job.ThreadGUID = row.ThreadGUID.String
	}
	if row.Context.Valid && row.Context.String != "" {
		var ctx types.JobContext
		if err := json.Unmarshal([]byte(row.Context.String), &ctx); err == nil {
			job.Context = &ctx
		}
	}
	return job
}
