package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jmcleod/edgefabric/internal/crypto"
	"github.com/jmcleod/edgefabric/internal/domain"
	"github.com/jmcleod/edgefabric/internal/storage"
	"github.com/jmcleod/edgefabric/internal/user"
)

// SeedSuperUser creates the initial superuser if no users exist in the database.
//
// Security:
//   - A random 16-character password is generated and logged once at startup.
//   - The password is stored only as a bcrypt hash — it is never persisted in plaintext.
//   - The operator should change this password immediately via the API.
func SeedSuperUser(ctx context.Context, userSvc user.Service, users storage.UserStore, logger *slog.Logger) error {
	// Check if any users exist.
	_, total, err := users.ListUsers(ctx, nil, storage.ListParams{Limit: 1})
	if err != nil {
		return err
	}
	if total > 0 {
		logger.Info("users already exist, skipping seed")
		return nil
	}

	// Generate a random password.
	password, err := crypto.GenerateRandomString(16)
	if err != nil {
		return err
	}

	_, err = userSvc.Create(ctx, user.CreateRequest{
		Email:    "admin@edgefabric.local",
		Name:     "Admin",
		Password: password,
		Role:     domain.RoleSuperUser,
		// TenantID is nil — superusers are not scoped to a tenant.
	})
	if err != nil {
		// If the user already exists (race condition), that's fine.
		if errors.Is(err, storage.ErrAlreadyExists) {
			logger.Info("seed superuser already exists")
			return nil
		}
		return err
	}

	logger.Warn("seed superuser created — change this password immediately",
		slog.String("email", "admin@edgefabric.local"),
		slog.String("password", password),
	)

	return nil
}