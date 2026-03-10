package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func (s *PostgresStore) CreateWireGuardPeer(ctx context.Context, p *domain.WireGuardPeer) error {
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.LastRotatedAt.IsZero() {
		p.LastRotatedAt = now
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO wireguard_peers (id, owner_type, owner_id, public_key, private_key, preshared_key, endpoint, last_rotated_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		p.ID.String(), string(p.OwnerType), p.OwnerID.String(),
		p.PublicKey, p.PrivateKey, nullStringEmpty(p.PresharedKey),
		nullStringEmpty(p.Endpoint), p.LastRotatedAt, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: wireguard peer already exists", storage.ErrAlreadyExists)
		}
		return fmt.Errorf("insert wireguard peer: %w", err)
	}

	// Insert allowed IPs.
	for _, ip := range p.AllowedIPs {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO wireguard_peer_allowed_ips (peer_id, allowed_ip) VALUES ($1, $2)`,
			p.ID.String(), ip,
		)
		if err != nil {
			return fmt.Errorf("insert allowed ip: %w", err)
		}
	}

	return tx.Commit()
}

func (s *PostgresStore) GetWireGuardPeer(ctx context.Context, id domain.ID) (*domain.WireGuardPeer, error) {
	p := &domain.WireGuardPeer{}
	var psk, endpoint sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT id, owner_type, owner_id, public_key, private_key, preshared_key, endpoint, last_rotated_at, created_at, updated_at
		 FROM wireguard_peers WHERE id = $1`, id.String(),
	).Scan(&p.ID, &p.OwnerType, &p.OwnerID, &p.PublicKey, &p.PrivateKey,
		&psk, &endpoint, &p.LastRotatedAt, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get wireguard peer: %w", err)
	}

	if psk.Valid {
		p.PresharedKey = psk.String
	}
	if endpoint.Valid {
		p.Endpoint = endpoint.String
	}

	ips, err := s.loadPeerAllowedIPs(ctx, id)
	if err != nil {
		return nil, err
	}
	p.AllowedIPs = ips

	return p, nil
}

func (s *PostgresStore) GetWireGuardPeerByOwner(ctx context.Context, ownerType domain.PeerOwnerType, ownerID domain.ID) (*domain.WireGuardPeer, error) {
	p := &domain.WireGuardPeer{}
	var psk, endpoint sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT id, owner_type, owner_id, public_key, private_key, preshared_key, endpoint, last_rotated_at, created_at, updated_at
		 FROM wireguard_peers WHERE owner_type = $1 AND owner_id = $2`,
		string(ownerType), ownerID.String(),
	).Scan(&p.ID, &p.OwnerType, &p.OwnerID, &p.PublicKey, &p.PrivateKey,
		&psk, &endpoint, &p.LastRotatedAt, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get wireguard peer by owner: %w", err)
	}

	if psk.Valid {
		p.PresharedKey = psk.String
	}
	if endpoint.Valid {
		p.Endpoint = endpoint.String
	}

	ips, err := s.loadPeerAllowedIPs(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	p.AllowedIPs = ips

	return p, nil
}

func (s *PostgresStore) ListWireGuardPeers(ctx context.Context, params storage.ListParams) ([]*domain.WireGuardPeer, int, error) {
	if params.Limit <= 0 {
		params.Limit = storage.DefaultLimit
	}

	var total int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM wireguard_peers`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count wireguard peers: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, owner_type, owner_id, public_key, private_key, preshared_key, endpoint, last_rotated_at, created_at, updated_at
		 FROM wireguard_peers ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		params.Limit, params.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list wireguard peers: %w", err)
	}
	defer rows.Close()

	var peers []*domain.WireGuardPeer
	for rows.Next() {
		p := &domain.WireGuardPeer{}
		var psk, endpoint sql.NullString
		if err := rows.Scan(&p.ID, &p.OwnerType, &p.OwnerID, &p.PublicKey, &p.PrivateKey,
			&psk, &endpoint, &p.LastRotatedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan wireguard peer: %w", err)
		}
		if psk.Valid {
			p.PresharedKey = psk.String
		}
		if endpoint.Valid {
			p.Endpoint = endpoint.String
		}
		peers = append(peers, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Load allowed IPs for each peer.
	for _, p := range peers {
		ips, err := s.loadPeerAllowedIPs(ctx, p.ID)
		if err != nil {
			return nil, 0, err
		}
		p.AllowedIPs = ips
	}

	return peers, total, nil
}

func (s *PostgresStore) UpdateWireGuardPeer(ctx context.Context, p *domain.WireGuardPeer) error {
	p.UpdatedAt = time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		`UPDATE wireguard_peers SET public_key = $1, private_key = $2, preshared_key = $3, endpoint = $4, last_rotated_at = $5, updated_at = $6
		 WHERE id = $7`,
		p.PublicKey, p.PrivateKey, nullStringEmpty(p.PresharedKey),
		nullStringEmpty(p.Endpoint), p.LastRotatedAt, p.UpdatedAt, p.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("update wireguard peer: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}

	// Replace allowed IPs: delete all, then reinsert.
	_, err = tx.ExecContext(ctx, `DELETE FROM wireguard_peer_allowed_ips WHERE peer_id = $1`, p.ID.String())
	if err != nil {
		return fmt.Errorf("delete old allowed ips: %w", err)
	}
	for _, ip := range p.AllowedIPs {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO wireguard_peer_allowed_ips (peer_id, allowed_ip) VALUES ($1, $2)`,
			p.ID.String(), ip,
		)
		if err != nil {
			return fmt.Errorf("insert allowed ip: %w", err)
		}
	}

	return tx.Commit()
}

func (s *PostgresStore) DeleteWireGuardPeer(ctx context.Context, id domain.ID) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM wireguard_peers WHERE id = $1`, id.String())
	if err != nil {
		return fmt.Errorf("delete wireguard peer: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// loadPeerAllowedIPs returns the allowed IPs for a WireGuard peer.
func (s *PostgresStore) loadPeerAllowedIPs(ctx context.Context, peerID domain.ID) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT allowed_ip FROM wireguard_peer_allowed_ips WHERE peer_id = $1`, peerID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("load allowed ips: %w", err)
	}
	defer rows.Close()

	var ips []string
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, fmt.Errorf("scan allowed ip: %w", err)
		}
		ips = append(ips, ip)
	}
	return ips, rows.Err()
}
