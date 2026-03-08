# Tenancy and RBAC

## Multi-Tenancy Model

EdgeFabric is multi-tenant by design. A **Tenant** is the top-level isolation boundary — all resources (nodes, gateways, DNS zones, CDN sites, routes, IP allocations, API keys, and users other than SuperUsers) belong to exactly one tenant.

### Tenant Lifecycle

```
active → suspended → deleted
         ↑           (soft delete: status flag, data retained)
         └── active
```

- **Active**: normal operation; all APIs functional.
- **Suspended**: read-only; mutations blocked at the RBAC layer. Useful for billing or abuse holds.
- **Deleted**: soft-deleted via status flag. Data is retained for audit purposes but inaccessible through the API. A future purge operation will hard-delete.

### Tenant Isolation Invariants

1. A non-SuperUser request **always** carries a tenant scope (derived from their JWT/API key claims).
2. The RBAC middleware rejects any request where `claims.TenantID != resource.TenantID`.
3. List endpoints automatically filter to the caller's tenant — there is no query parameter to list across tenants except for SuperUser.
4. Nodes and IP prefixes assigned to a tenant are exclusive; they cannot be shared.

### Tenant Configuration

Each tenant has a `settings` JSON column for tenant-level overrides (rate limits, feature flags, default TTLs, etc.). The schema is intentionally freeform in v1 — structured tenant settings will be added when specific features require them.

---

## Roles

EdgeFabric uses a flat three-role model. Roles are assigned per-user and per-API-key.

| Role | Scope | Capabilities |
|------|-------|--------------|
| **SuperUser** | Global (no tenant) | Full access to everything: create/manage tenants, create users in any tenant, global fleet operations. |
| **Admin** | Single tenant | Full CRUD within their tenant: users, nodes, gateways, DNS, CDN, routes, API keys. Cannot create tenants or access other tenants. |
| **ReadOnly** | Single tenant | Read and list only within their tenant. Cannot create, update, or delete any resource. |

### Role Assignment Rules

- A **SuperUser** has `tenant_id = NULL` — they are not scoped to any tenant.
- An **Admin** or **ReadOnly** user must have a non-null `tenant_id`.
- API keys inherit a role at creation time. An Admin can create API keys with Admin or ReadOnly role.
- SuperUser API keys are not supported (API keys are always tenant-scoped).

---

## RBAC Enforcement

Authorization is enforced by the `rbac.Authorizer` interface, checked in the `RequirePermission` HTTP middleware. Every protected endpoint declares its required action and resource type at registration time.

### Actions

```
create | read | update | delete | list
```

### Resources

```
tenant | user | node | node_group | gateway | ip_allocation |
bgp_session | dns_zone | dns_record | cdn_site | cdn_origin |
route | ssh_key | tls_certificate | audit_event | api_key
```

### Authorization Rules

1. **SuperUser** → allowed for all actions on all resources.
2. **ReadOnly** → allowed only for `read` and `list` actions.
3. **Admin** → allowed for all actions, subject to tenant scope.
4. **Tenant scope check**: for non-SuperUser, `claims.TenantID` must match the resource's tenant ID. If there is a mismatch → `403 Forbidden`.
5. **Global resources** (`tenant`, `ssh_key`): non-SuperUser users can only `read`/`list`, never `create`/`update`/`delete`.

### Request Flow

```
Request → Auth Middleware (extract claims from token/API key)
        → RBAC Middleware (Authorizer.Authorize(claims, action, resource, tenantID))
        → Handler (business logic)
```

If either middleware rejects the request, it returns a JSON error envelope:
- `401 Unauthorized` — missing or invalid authentication.
- `403 Forbidden` — authenticated but insufficient permissions.

---

## Authentication Methods

### Password + Session Token

1. `POST /api/v1/auth/login` with `{email, password}`.
2. If TOTP is not enabled → returns `{token, totp_required: false}`.
3. If TOTP is enabled → returns `{token, totp_required: true}`. Client must call `POST /api/v1/auth/totp/verify` with the code.
4. The session token is an HMAC-SHA256 signed payload (not JWT) containing `userID|tenantID|role|issuedAt|expiresAt`. The same process issues and verifies it, so asymmetric signing is unnecessary.

### API Key

1. API keys are created via `POST /api/v1/api-keys` with `{name, role}`.
2. The raw key is returned once and cannot be retrieved again.
3. Authenticate by sending `Authorization: Bearer ef_<key>`.
4. The `ef_` prefix distinguishes API keys from session tokens.
5. API keys are looked up by prefix (first 8 characters) and verified via bcrypt comparison of the full key.

### TOTP 2FA

1. Enroll: `POST /api/v1/auth/totp/enroll` → returns secret and provisioning URI.
2. Confirm: `POST /api/v1/auth/totp/confirm` with a valid code → enables TOTP.
3. Verify: after a TOTP-required login, `POST /api/v1/auth/totp/verify` with the code → issues a full session token.
4. TOTP secrets are encrypted at rest using AES-256-GCM before storage.

---

## Audit Trail

Every state-changing operation (create, update, delete) emits an audit event containing:

- Who (user ID or API key ID)
- What (action + resource type)
- When (timestamp)
- Where (source IP)
- Details (JSON with entity-specific context)

Audit events are append-only and immutable. They are tenant-scoped for non-SuperUser queries and globally visible to SuperUsers.
