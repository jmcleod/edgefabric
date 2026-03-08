import { cn } from '@/lib/utils';
import { LucideIcon } from 'lucide-react';

interface StatCardProps {
  title: string;
  value: string | number;
  subtitle?: string;
  icon?: LucideIcon;
  trend?: {
    value: number;
    label?: string;
    positive?: boolean;
  };
  variant?: 'default' | 'healthy' | 'warning' | 'critical';
}

export function StatCard({ title, value, subtitle, icon: Icon, trend, variant = 'default' }: StatCardProps) {
  return (
    <div className="rounded-lg border border-border bg-card p-5">
      <div className="flex items-start justify-between">
        <div className="space-y-1">
          <p className="text-sm font-medium text-muted-foreground">{title}</p>
          <p className={cn(
            'text-2xl font-semibold tracking-tight',
            variant === 'healthy' && 'text-status-healthy',
            variant === 'warning' && 'text-status-warning',
            variant === 'critical' && 'text-status-critical'
          )}>
            {value}
          </p>
          {subtitle && (
            <p className="text-sm text-muted-foreground">{subtitle}</p>
          )}
          {trend && (
            <p className={cn(
              'text-xs font-medium',
              trend.positive ? 'text-status-healthy' : 'text-status-critical'
            )}>
              {trend.positive ? '+' : ''}{trend.value}%
              {trend.label && <span className="text-muted-foreground"> {trend.label}</span>}
            </p>
          )}
        </div>
        {Icon && (
          <div className={cn(
            'flex h-10 w-10 items-center justify-center rounded-lg',
            variant === 'default' && 'bg-primary/10 text-primary',
            variant === 'healthy' && 'bg-status-healthy/10 text-status-healthy',
            variant === 'warning' && 'bg-status-warning/10 text-status-warning',
            variant === 'critical' && 'bg-status-critical/10 text-status-critical'
          )}>
            <Icon className="h-5 w-5" />
          </div>
        )}
      </div>
    </div>
  );
}
