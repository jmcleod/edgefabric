package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *SQLiteStore) CreateBGPSession(ctx context.Context, sess *domain.BGPSession) error {
	now := time.Now().UTC()
	sess.CreatedAt = now
	sess.UpdatedAt = now
	if sess.Status == "" {
		sess.Status = domain.BGPSessionConfigured
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	metadata := sql.NullString{}
	if sess.Metadata != nil {
		metadata = sql.NullString{String: string(sess.Metadata), Valid: true}
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO bgp_sessions (id, node_id, peer_asn, peer_address, local_asn, status, import_policy, export_policy, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID.String(), sess.NodeID.String(), sess.PeerASN, sess.PeerAddress,
		sess.LocalASN, string(sess.Status),
		nullStringEmpty(sess.ImportPolicy), nullStringEmpty(sess.ExportPolicy),
		metadata, sess.CreatedAt, sess.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: bgp session already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert bgp session: %w", err)
	}

	// Insert announced prefixes.
	for _, prefix := range sess.AnnouncedPrefixes {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO bgp_announced_prefixes (session_id, prefix) VALUES (?, ?)`,
			sess.ID.String(), prefix,
		)
		if err != nil {
			return fmt.Errorf("insert announced prefix: %w", err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) GetBGPSession(ctx context.Context, id domain.ID) (*domain.BGPSession, error) {
	sess := &domain.BGPSession{}
	var importPolicy, exportPolicy, metadata sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT id, node_id, peer_asn, peer_address, local_asn, status, import_policy, export_policy, metadata, created_at, updated_at
		 FROM bgp_sessions WHERE id = ?`, id.String(),
	).Scan(&sess.ID, &sess.NodeID, &sess.PeerASN, &sess.PeerAddress,
		&sess.LocalASN, &sess.Status, &importPolicy, &exportPolicy,
		&metadata, &sess.CreatedAt, &sess.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get bgp session: %w", err)
	}

	if importPolicy.Valid {
		sess.ImportPolicy = importPolicy.String
	}
	if exportPolicy.Valid {
		sess.ExportPolicy = exportPolicy.String
	}
	if metadata.Valid {
		sess.Metadata = json.RawMessage(metadata.String)
	}

	prefixes, err := s.loadSessionAnnouncedPrefixes(ctx, id)
	if err != nil {
		return nil, err
	}
	sess.AnnouncedPrefixes = prefixes

	return sess, nil
}

func (s *SQLiteStore) ListBGPSessions(ctx context.Context, nodeID domain.ID, params storage.ListParams) ([]*domain.BGPSession, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM bgp_sessions WHERE node_id = ?`, nodeID.String(),
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count bgp sessions: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, node_id, peer_asn, peer_address, local_asn, status, import_policy, export_policy, metadata, created_at, updated_at
		 FROM bgp_sessions WHERE node_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		nodeID.String(), params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list bgp sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*domain.BGPSession
	for rows.Next() {
		sess := &domain.BGPSession{}
		var importPolicy, exportPolicy, metadata sql.NullString
		if err := rows.Scan(&sess.ID, &sess.NodeID, &sess.PeerASN, &sess.PeerAddress,
			&sess.LocalASN, &sess.Status, &importPolicy, &exportPolicy,
			&metadata, &sess.CreatedAt, &sess.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan bgp session: %w", err)
		}
		if importPolicy.Valid {
			sess.ImportPolicy = importPolicy.String
		}
		if exportPolicy.Valid {
			sess.ExportPolicy = exportPolicy.String
		}
		if metadata.Valid {
			sess.Metadata = json.RawMessage(metadata.String)
		}
		sessions = append(sessions, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Load announced prefixes for each session.
	for _, sess := range sessions {
		prefixes, err := s.loadSessionAnnouncedPrefixes(ctx, sess.ID)
		if err != nil {
			return nil, 0, err
		}
		sess.AnnouncedPrefixes = prefixes
	}

	return sessions, total, nil
}

func (s *SQLiteStore) UpdateBGPSession(ctx context.Context, sess *domain.BGPSession) error {
	sess.UpdatedAt = time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	metadata := sql.NullString{}
	if sess.Metadata != nil {
		metadata = sql.NullString{String: string(sess.Metadata), Valid: true}
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE bgp_sessions SET peer_asn = ?, peer_address = ?, local_asn = ?, status = ?,
		 import_policy = ?, export_policy = ?, metadata = ?, updated_at = ?
		 WHERE id = ?`,
		sess.PeerASN, sess.PeerAddress, sess.LocalASN, string(sess.Status),
		nullStringEmpty(sess.ImportPolicy), nullStringEmpty(sess.ExportPolicy),
		metadata, sess.UpdatedAt, sess.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update bgp session: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}

	// Replace announced prefixes.
	_, err = tx.ExecContext(ctx, `DELETE FROM bgp_announced_prefixes WHERE session_id = ?`, sess.ID.String())
	if err != nil {
		return fmt.Errorf("delete old prefixes: %w", err)
	}
	for _, prefix := range sess.AnnouncedPrefixes {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO bgp_announced_prefixes (session_id, prefix) VALUES (?, ?)`,
			sess.ID.String(), prefix,
		)
		if err != nil {
			return fmt.Errorf("insert announced prefix: %w", err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) DeleteBGPSession(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM bgp_sessions WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete bgp session: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// loadSessionAnnouncedPrefixes returns the announced prefixes for a BGP session.
func (s *SQLiteStore) loadSessionAnnouncedPrefixes(ctx context.Context, sessionID domain.ID) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT prefix FROM bgp_announced_prefixes WHERE session_id = ?`, sessionID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("load announced prefixes: %w", err)
	}
	defer rows.Close()

	var prefixes []string
	for rows.Next() {
		var prefix string
		if err := rows.Scan(&prefix); err != nil {
			return nil, fmt.Errorf("scan prefix: %w", err)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, rows.Err()
}
