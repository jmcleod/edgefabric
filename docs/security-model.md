# EdgeFabric Security Model

This document describes the cryptographic primitives, authentication mechanisms, and security assumptions in EdgeFabric.

## Encryption at Rest

- **Algorithm**: AES-256-GCM (authenticated encryption with associated data)
- **Key**: 32-byte key, provided as base64 in `controller.secrets.encryption_key`
- **Nonce**: Random 12-byte nonce generated per encryption operation
- **Scope**: Protects TOTP secrets and SSH key passphrases stored in SQLite
- **Implementation**: `internal/crypto/crypto.go`

The encryption key is validated at startup: it must be valid base64 decoding to exactly 32 bytes. An empty key is permitted (the secrets store will operate without encryption).

## Token Signing

- **Algorithm**: HMAC-SHA256 (symmetric)
- **Key**: Configurable via `controller.secrets.token_signing_key`; falls back to `encryption_key` if not set
- **TTL**: 24 hours
- **Claims**: UserID, TenantID, Role
- **Implementation**: `internal/auth/token.go`

### Key Separation

Production deployments should use a separate `token_signing_key` to limit blast radius: if the signing key is compromised, only session tokens are affected (not encrypted secrets). If `token_signing_key` is not configured, the controller logs a warning and uses the encryption key as a fallback for backward compatibility.

## Password Hashing

- **Algorithm**: bcrypt
- **Cost factor**: 12
- **Implementation**: `internal/auth/auth.go`

Passwords are never stored in plaintext. The bcrypt hash is stored in the `users.password_hash` column.

## API Key Scheme

- **Format**: `efk_<prefix>_<random>` — prefix enables lookup without scanning all keys
- **Storage**: bcrypt hash of the full key; only the prefix is stored in plaintext
- **Lookup**: Find by prefix, then verify with bcrypt
- **Implementation**: `internal/auth/apikey.go`

The raw API key is shown exactly once at creation time. It cannot be recovered.

## TOTP (Two-Factor Authentication)

- **Algorithm**: TOTP (RFC 6238), 6-digit codes, 30-second window
- **Secret storage**: AES-256-GCM encrypted at rest
- **Enrollment flow**: Generate secret → user scans QR → user confirms with valid code → TOTP enabled
- **Implementation**: `internal/auth/totp.go`

## Transport Security

- **Node/Gateway ↔ Controller**: WireGuard overlay network (Noise protocol, Curve25519, ChaCha20-Poly1305)
- **API clients ↔ Controller**: TLS recommended; no TLS enforcement in v1 (operator responsibility)
- **Inter-node**: All traffic traverses the WireGuard mesh

## Audit Logging

All security-sensitive operations are logged to the `audit_events` table:
- Login success and failure (failed logins do not reveal whether the email exists)
- TOTP enrollment, verification success and failure
- API key creation and deletion
- User/tenant CRUD operations
- Node/gateway provisioning actions

Audit events include: tenant ID, user ID, action, resource, source IP, and timestamp.

## RBAC

- **Model**: Role-based access control with tenant isolation
- **Roles**: `superuser`, `admin`, `operator`, `viewer`
- **Enforcement**: Middleware checks on every protected endpoint
- **Tenant scoping**: Non-superuser requests are always scoped to their tenant
- **Implementation**: `internal/rbac/`

## Assumptions

1. **Operator protects config**: The config file contains encryption keys and must be protected by file system permissions.
2. **Enrollment tokens are one-time**: Used during node/gateway enrollment, then marked as consumed.
3. **WireGuard keys are generated server-side**: Private keys are encrypted at rest and transmitted over the WireGuard tunnel during enrollment.

## Implemented Security Features

- **Rate limiting on auth endpoints**: Token-bucket rate limiting protects login and TOTP endpoints against brute-force attacks. Implementation: `internal/api/middleware/ratelimit.go`.
- **Key rotation with versioned IDs**: Encryption keys support versioned key IDs, allowing seamless rotation without re-encrypting all secrets at once. Implementation: `internal/crypto/crypto.go`, `internal/secrets/store.go`.
- **CORS**: Configurable CORS middleware for cross-origin SPA deployments. Implementation: `internal/api/middleware/cors.go`.
- **TLS auto-renewal**: ACME/Let's Encrypt integration via `autocert.Manager` for automatic TLS certificate management. Implementation: `internal/app/controller.go`.
- **HA controller**: PostgreSQL backend (`internal/storage/postgres/`) with leader election via PostgreSQL advisory locks (`internal/ha/leader.go`). SQLite remains the default for single-instance deployments.

## Known Limitations / Future Work

- **HSTS**: Not enforced. Will be added when TLS is mandatory for all deployments.
- **Mutual TLS**: Client certificate authentication for node-to-controller communication is not yet implemented.
