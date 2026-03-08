package sqlite_test

import (
	"context"
	"testing"

	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
)

func TestSSHKeyCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	key := &domain.SSHKey{
		ID:          domain.NewID(),
		Name:        "deploy-key-1",
		PublicKey:    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest",
		PrivateKey:  "encrypted-private-key-data",
		Fingerprint: "SHA256:abc123",
	}
	if err := store.CreateSSHKey(ctx, key); err != nil {
		t.Fatalf("create ssh key: %v", err)
	}

	if key.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}

	// Get.
	got, err := store.GetSSHKey(ctx, key.ID)
	if err != nil {
		t.Fatalf("get ssh key: %v", err)
	}
	if got.Name != "deploy-key-1" {
		t.Errorf("expected name deploy-key-1, got %s", got.Name)
	}
	if got.PrivateKey != "encrypted-private-key-data" {
		t.Error("private key should be retrievable from store (encrypted at rest)")
	}

	// List.
	keys, total, err := store.ListSSHKeys(ctx, storage.ListParams{Limit: 10})
	if err != nil {
		t.Fatalf("list ssh keys: %v", err)
	}
	if total != 1 || len(keys) != 1 {
		t.Errorf("expected 1 key, got total=%d len=%d", total, len(keys))
	}

	// Delete.
	if err := store.DeleteSSHKey(ctx, key.ID); err != nil {
		t.Fatalf("delete ssh key: %v", err)
	}
	_, err = store.GetSSHKey(ctx, key.ID)
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSSHKeyNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetSSHKey(ctx, domain.NewID())
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
