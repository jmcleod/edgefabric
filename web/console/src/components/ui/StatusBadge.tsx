import { cn } from '@/lib/utils';
import type { HealthStatus } from '@/types';

interface StatusBadgeProps {
  status: HealthStatus | 'active' | 'suspended' | 'pending' | 'established' | 'idle' | 'connect' | 'leader' | 'follower';
  size?: 'sm' | 'md';
  showDot?: boolean;
}

const statusConfig: Record<string, { label: string; className: string }> = {
  healthy: { label: 'Healthy', className: 'status-healthy' },
  warning: { label: 'Warning', className: 'status-warning' },
  critical: { label: 'Critical', className: 'status-critical' },
  unknown: { label: 'Unknown', className: 'status-unknown' },
  syncing: { label: 'Syncing', className: 'status-syncing' },
  active: { label: 'Active', className: 'status-healthy' },
  suspended: { label: 'Suspended', className: 'status-critical' },
  pending: { label: 'Pending', className: 'status-warning' },
  established: { label: 'Established', className: 'status-healthy' },
  idle: { label: 'Idle', className: 'status-warning' },
  connect: { label: 'Connecting', className: 'status-syncing' },
  leader: { label: 'Leader', className: 'status-healthy' },
  follower: { label: 'Follower', className: 'status-warning' },
};

export function StatusBadge({ status, size = 'md', showDot = true }: StatusBadgeProps) {
  const config = statusConfig[status] || statusConfig.unknown;

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full border font-medium',
        config.className,
        size === 'sm' ? 'px-2 py-0.5 text-xs' : 'px-2.5 py-1 text-xs'
      )}
    >
      {showDot && (
        <span
          className={cn(
            'rounded-full',
            size === 'sm' ? 'h-1.5 w-1.5' : 'h-2 w-2',
            status === 'syncing' && 'animate-pulse-status'
          )}
          style={{ backgroundColor: 'currentColor' }}
        />
      )}
      {config.label}
    </span>
  );
}
