// Package ha provides high-availability primitives for EdgeFabric controllers.
package ha

import (
	"context"
	"database/sql"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jmcleod/edgefabric/internal/events"
)

// defaultLockID is the PostgreSQL advisory lock ID used for leader election.
// This is a fixed constant — all EdgeFabric controllers use the same lock ID.
const defaultLockID int64 = 0x45464C4452 // "EFLDR"

// defaultInterval is the default time between lock check attempts.
const defaultInterval = 5 * time.Second

// LeaderElector manages leader election for HA controller deployments.
type LeaderElector interface {
	// Start begins the election loop. Blocks until ctx is cancelled.
	Start(ctx context.Context) error
	// IsLeader returns whether this instance currently holds the leader lock.
	IsLeader() bool
	// Stop gracefully releases the leader lock.
	Stop()
}

// --- PostgreSQL Advisory Lock Elector ---

// PostgresLeaderElector uses pg_try_advisory_lock for session-scoped leader election.
// The lock is automatically released if the holding connection drops (crash safety).
type PostgresLeaderElector struct {
	db         *sql.DB
	logger     *slog.Logger
	bus        *events.Bus
	isLeader   atomic.Bool
	onElected  func()
	onDemoted  func()
	interval   time.Duration
	lockID     int64
	cancel     context.CancelFunc
	done       chan struct{}
}

// Option configures a PostgresLeaderElector.
type Option func(*PostgresLeaderElector)

// WithElectionInterval sets the time between lock check attempts.
func WithElectionInterval(d time.Duration) Option {
	return func(e *PostgresLeaderElector) {
		e.interval = d
	}
}

// WithLockID sets the PostgreSQL advisory lock ID.
func WithLockID(id int64) Option {
	return func(e *PostgresLeaderElector) {
		e.lockID = id
	}
}

// NewPostgresLeaderElector creates a leader elector using PostgreSQL advisory locks.
func NewPostgresLeaderElector(
	db *sql.DB,
	logger *slog.Logger,
	bus *events.Bus,
	onElected, onDemoted func(),
	opts ...Option,
) *PostgresLeaderElector {
	e := &PostgresLeaderElector{
		db:        db,
		logger:    logger,
		bus:       bus,
		onElected: onElected,
		onDemoted: onDemoted,
		interval:  defaultInterval,
		lockID:    defaultLockID,
		done:      make(chan struct{}),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Start begins the election loop. It acquires a dedicated database connection
// and periodically attempts to obtain the advisory lock. Blocks until ctx is cancelled.
func (e *PostgresLeaderElector) Start(ctx context.Context) error {
	ctx, e.cancel = context.WithCancel(ctx)
	defer close(e.done)

	// Acquire a dedicated connection from the pool. Advisory locks are
	// session-scoped, so we must reuse the same connection for all lock ops.
	conn, err := e.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	// Try to acquire immediately, then on each tick.
	e.tryAcquire(ctx, conn)

	for {
		select {
		case <-ctx.Done():
			// Release lock on shutdown.
			if e.isLeader.Load() {
				e.releaseLock(conn)
				e.demote()
			}
			return nil
		case <-ticker.C:
			e.tryAcquire(ctx, conn)
		}
	}
}

// IsLeader returns whether this instance currently holds the leader lock.
func (e *PostgresLeaderElector) IsLeader() bool {
	return e.isLeader.Load()
}

// Stop gracefully releases the leader lock and stops the election loop.
func (e *PostgresLeaderElector) Stop() {
	if e.cancel != nil {
		e.cancel()
		<-e.done
	}
}

func (e *PostgresLeaderElector) tryAcquire(ctx context.Context, conn *sql.Conn) {
	var acquired bool
	err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", e.lockID).Scan(&acquired)
	if err != nil {
		// Connection error — treat as demotion.
		if e.isLeader.Load() {
			e.logger.Warn("leader lock connection error, demoting", slog.String("error", err.Error()))
			e.demote()
		}
		return
	}

	wasLeader := e.isLeader.Load()
	if acquired && !wasLeader {
		e.elect()
	} else if !acquired && wasLeader {
		// Lock lost (e.g., connection was reset by PG and lock released).
		e.demote()
	}
}

func (e *PostgresLeaderElector) elect() {
	e.isLeader.Store(true)
	e.logger.Info("elected as leader")
	if e.onElected != nil {
		e.onElected()
	}
	if e.bus != nil {
		e.bus.Publish(context.Background(), events.Event{
			Type:      events.LeaderElected,
			Timestamp: time.Now(),
			Severity:  events.SeverityInfo,
			Resource:  "controller",
			Details:   map[string]string{"action": "elected"},
		})
	}
}

func (e *PostgresLeaderElector) demote() {
	e.isLeader.Store(false)
	e.logger.Info("demoted from leader")
	if e.onDemoted != nil {
		e.onDemoted()
	}
	if e.bus != nil {
		e.bus.Publish(context.Background(), events.Event{
			Type:      events.LeaderLost,
			Timestamp: time.Now(),
			Severity:  events.SeverityWarning,
			Resource:  "controller",
			Details:   map[string]string{"action": "demoted"},
		})
	}
}

func (e *PostgresLeaderElector) releaseLock(conn *sql.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", e.lockID); err != nil {
		e.logger.Warn("failed to release leader lock", slog.String("error", err.Error()))
	}
}

// --- No-op Elector (SQLite / single-instance) ---

// NoopLeaderElector is a leader elector that always reports as leader.
// Used for SQLite mode where only one controller instance exists.
type NoopLeaderElector struct {
	isLeader  atomic.Bool
	onElected func()
	onDemoted func()
}

// NewNoopLeaderElector creates a no-op leader elector that always reports as leader.
func NewNoopLeaderElector(onElected, onDemoted func()) *NoopLeaderElector {
	return &NoopLeaderElector{
		onElected: onElected,
		onDemoted: onDemoted,
	}
}

// Start immediately calls onElected and blocks until ctx is cancelled.
func (e *NoopLeaderElector) Start(ctx context.Context) error {
	e.isLeader.Store(true)
	if e.onElected != nil {
		e.onElected()
	}
	<-ctx.Done()
	e.isLeader.Store(false)
	if e.onDemoted != nil {
		e.onDemoted()
	}
	return nil
}

// IsLeader always returns true for single-instance mode.
func (e *NoopLeaderElector) IsLeader() bool {
	return e.isLeader.Load()
}

// Stop is a no-op for the noop elector.
func (e *NoopLeaderElector) Stop() {}
