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

1. **Single-instance controller**: No HA, no distributed consensus. The SQLite database is the single source of truth.
2. **Operator protects config**: The config file contains encryption keys and must be protected by file system permissions.
3. **Enrollment tokens are one-time**: Used during node/gateway enrollment, then marked as consumed.
4. **WireGuard keys are generated server-side**: Private keys are encrypted at rest and transmitted over the WireGuard tunnel during enrollment.

## Known Limitations / Future Work

- **Rate limiting on auth endpoints**: Not yet implemented. Brute-force protection relies on bcrypt cost.
- **Key rotation**: Not supported in v1. Changing the encryption key requires re-encrypting all secrets. Planned: versioned key IDs.
- **CORS**: Not configured. Will be needed when cross-origin SPA deployments are supported.
- **HSTS**: Not enforced. Will be added when TLS is mandatory.
- **Certificate auto-renewal**: TLS certificate management is manual in v1.
- **HA controller**: PostgreSQL backend and leader election are planned for v2.
