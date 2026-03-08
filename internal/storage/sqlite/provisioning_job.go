package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *SQLiteStore) CreateProvisioningJob(ctx context.Context, j *domain.ProvisioningJob) error {
	now := time.Now().UTC()
	j.CreatedAt = now
	j.UpdatedAt = now
	if j.Status == "" {
		j.Status = domain.ProvisionStatusPending
	}

	steps := sql.NullString{}
	if j.Steps != nil {
		steps = sql.NullString{String: string(j.Steps), Valid: true}
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO provisioning_jobs (id, node_id, tenant_id, action, status, current_step, steps, error, initiated_by, started_at, completed_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID.String(), j.NodeID.String(), nullIDString(j.TenantID),
		string(j.Action), string(j.Status), string(j.CurrentStep),
		steps, nullStringEmpty(j.Error), j.InitiatedBy.String(),
		nullTime(j.StartedAt), nullTime(j.CompletedAt),
		j.CreatedAt, j.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: provisioning job already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert provisioning job: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetProvisioningJob(ctx context.Context, id domain.ID) (*domain.ProvisioningJob, error) {
	j := &domain.ProvisioningJob{}
	var tenantID, stepsJSON, errStr sql.NullString
	var startedAt, completedAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, node_id, tenant_id, action, status, current_step, steps, error, initiated_by, started_at, completed_at, created_at, updated_at
		 FROM provisioning_jobs WHERE id = ?`, id.String(),
	).Scan(&j.ID, &j.NodeID, &tenantID, &j.Action, &j.Status, &j.CurrentStep,
		&stepsJSON, &errStr, &j.InitiatedBy, &startedAt, &completedAt,
		&j.CreatedAt, &j.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get provisioning job: %w", err)
	}

	applyNullableJobFields(j, tenantID, stepsJSON, errStr, startedAt, completedAt)
	return j, nil
}

func (s *SQLiteStore) ListProvisioningJobs(ctx context.Context, nodeID *domain.ID, params storage.ListParams) ([]*domain.ProvisioningJob, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	var countErr error
	if nodeID != nil {
		countErr = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM provisioning_jobs WHERE node_id = ?`, nodeID.String(),
		).Scan(&total)
	} else {
		countErr = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM provisioning_jobs`,
		).Scan(&total)
	}
	if countErr != nil {
		return nil, 0, fmt.Errorf("count provisioning jobs: %w", countErr)
	}

	var rows *sql.Rows
	var err error
	if nodeID != nil {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, node_id, tenant_id, action, status, current_step, steps, error, initiated_by, started_at, completed_at, created_at, updated_at
			 FROM provisioning_jobs WHERE node_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
			nodeID.String(), params.Limit, params.Offset,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, node_id, tenant_id, action, status, current_step, steps, error, initiated_by, started_at, completed_at, created_at, updated_at
			 FROM provisioning_jobs ORDER BY created_at DESC LIMIT ? OFFSET ?`,
			params.Limit, params.Offset,
		)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("list provisioning jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*domain.ProvisioningJob
	for rows.Next() {
		j := &domain.ProvisioningJob{}
		var tenantID, stepsJSON, errStr sql.NullString
		var startedAt, completedAt sql.NullTime

		if err := rows.Scan(&j.ID, &j.NodeID, &tenantID, &j.Action, &j.Status, &j.CurrentStep,
			&stepsJSON, &errStr, &j.InitiatedBy, &startedAt, &completedAt,
			&j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan provisioning job: %w", err)
		}
		applyNullableJobFields(j, tenantID, stepsJSON, errStr, startedAt, completedAt)
		jobs = append(jobs, j)
	}
	return jobs, total, rows.Err()
}

func (s *SQLiteStore) UpdateProvisioningJob(ctx context.Context, j *domain.ProvisioningJob) error {
	j.UpdatedAt = time.Now().UTC()

	steps := sql.NullString{}
	if j.Steps != nil {
		steps = sql.NullString{String: string(j.Steps), Valid: true}
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE provisioning_jobs SET status = ?, current_step = ?, steps = ?, error = ?,
		 started_at = ?, completed_at = ?, updated_at = ?
		 WHERE id = ?`,
		string(j.Status), string(j.CurrentStep), steps, nullStringEmpty(j.Error),
		nullTime(j.StartedAt), nullTime(j.CompletedAt), j.UpdatedAt, j.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update provisioning job: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) GetActiveProvisioningJob(ctx context.Context, nodeID domain.ID) (*domain.ProvisioningJob, error) {
	j := &domain.ProvisioningJob{}
	var tenantID, stepsJSON, errStr sql.NullString
	var startedAt, completedAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, node_id, tenant_id, action, status, current_step, steps, error, initiated_by, started_at, completed_at, created_at, updated_at
		 FROM provisioning_jobs WHERE node_id = ? AND status IN ('pending', 'running')
		 ORDER BY created_at DESC LIMIT 1`, nodeID.String(),
	).Scan(&j.ID, &j.NodeID, &tenantID, &j.Action, &j.Status, &j.CurrentStep,
		&stepsJSON, &errStr, &j.InitiatedBy, &startedAt, &completedAt,
		&j.CreatedAt, &j.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get active provisioning job: %w", err)
	}

	applyNullableJobFields(j, tenantID, stepsJSON, errStr, startedAt, completedAt)
	return j, nil
}

// nullTime converts a *time.Time to sql.NullTime.
func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// applyNullableJobFields applies nullable SQL scan results to a ProvisioningJob.
func applyNullableJobFields(j *domain.ProvisioningJob, tenantID, stepsJSON, errStr sql.NullString, startedAt, completedAt sql.NullTime) {
	if tenantID.Valid {
		id, err := uuid.Parse(tenantID.String)
		if err == nil {
			j.TenantID = &id
		}
	}
	if stepsJSON.Valid {
		j.Steps = json.RawMessage(stepsJSON.String)
	}
	if errStr.Valid {
		j.Error = errStr.String
	}
	if startedAt.Valid {
		j.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		j.CompletedAt = &completedAt.Time
	}
}
