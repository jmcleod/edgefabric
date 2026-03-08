package sqlite_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestBGPSessionCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	tenantID := createTestTenant(t, store)
	nodeID := createTestNode(t, store, &tenantID)

	sess := &domain.BGPSession{
		ID:                domain.NewID(),
		NodeID:            nodeID,
		PeerASN:           65001,
		PeerAddress:       "198.51.100.1",
		LocalASN:          65000,
		AnnouncedPrefixes: []string{"203.0.113.0/24", "198.51.100.0/24"},
		ImportPolicy:      "accept-all",
		ExportPolicy:      "export-default",
	}

	// Create.
	if err := store.CreateBGPSession(ctx, sess); err != nil {
		t.Fatalf("create bgp session: %v", err)
	}
	if sess.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
	if sess.Status != domain.BGPSessionConfigured {
		t.Errorf("expected default status configured, got %s", sess.Status)
	}

	// Get by ID.
	got, err := store.GetBGPSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get bgp session: %v", err)
	}
	if got.PeerASN != 65001 {
		t.Errorf("expected peer_asn 65001, got %d", got.PeerASN)
	}
	if got.PeerAddress != "198.51.100.1" {
		t.Errorf("expected peer_address 198.51.100.1, got %s", got.PeerAddress)
	}
	if got.LocalASN != 65000 {
		t.Errorf("expected local_asn 65000, got %d", got.LocalASN)
	}
	if len(got.AnnouncedPrefixes) != 2 {
		t.Errorf("expected 2 announced prefixes, got %d", len(got.AnnouncedPrefixes))
	}
	if got.ImportPolicy != "accept-all" {
		t.Errorf("expected import_policy accept-all, got %s", got.ImportPolicy)
	}
	if got.ExportPolicy != "export-default" {
		t.Errorf("expected export_policy export-default, got %s", got.ExportPolicy)
	}

	// List by node.
	sessions, total, err := store.ListBGPSessions(ctx, nodeID, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list bgp sessions: %v", err)
	}
	if total != 1 || len(sessions) != 1 {
		t.Errorf("expected 1 session, got total=%d len=%d", total, len(sessions))
	}

	// Update — change status and prefixes.
	got.Status = domain.BGPSessionEstablished
	got.AnnouncedPrefixes = []string{"203.0.113.0/24"}
	if err := store.UpdateBGPSession(ctx, got); err != nil {
		t.Fatalf("update bgp session: %v", err)
	}

	updated, err := store.GetBGPSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if updated.Status != domain.BGPSessionEstablished {
		t.Errorf("expected status established, got %s", updated.Status)
	}
	if len(updated.AnnouncedPrefixes) != 1 {
		t.Errorf("expected 1 prefix after update, got %d", len(updated.AnnouncedPrefixes))
	}

	// Delete.
	if err := store.DeleteBGPSession(ctx, sess.ID); err != nil {
		t.Fatalf("delete bgp session: %v", err)
	}
	_, err = store.GetBGPSession(ctx, sess.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestBGPSessionNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetBGPSession(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestBGPSessionDeleteNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.DeleteBGPSession(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
