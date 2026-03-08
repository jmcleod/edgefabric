import { AppLayout } from '@/components/layout/AppLayout';
import { StatCard } from '@/components/ui/StatCard';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { DataTable, Column } from '@/components/ui/DataTable';
import { PageHeader } from '@/components/ui/PageHeader';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { useStatus } from '@/hooks/useStatus';
import { useNodes } from '@/hooks/useNodes';
import { useAuditLogs } from '@/hooks/useAudit';
import { useProvisioningJobs } from '@/hooks/useProvisioning';
import type { Node, ProvisioningJob, AuditLogEntry } from '@/types';
import {
  Server,
  Building2,
  Globe,
  Layers,
  Activity,
  HardDrive,
  AlertTriangle,
  CheckCircle,
} from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';

const nodeColumns: Column<Node>[] = [
  {
    key: 'name',
    header: 'Node',
    render: (node) => (
      <div>
        <p className="font-medium text-foreground">{node.name}</p>
        <p className="text-xs text-muted-foreground mono-data">{node.ipv4}</p>
      </div>
    ),
  },
  { key: 'region', header: 'Region' },
  {
    key: 'status',
    header: 'Status',
    render: (node) => <StatusBadge status={node.status} size="sm" />,
  },
  {
    key: 'lastSeen',
    header: 'Last Seen',
    render: (node) => (
      <span className="text-muted-foreground text-sm">
        {node.lastSeen
          ? formatDistanceToNow(new Date(node.lastSeen), { addSuffix: true })
          : '\u2014'}
      </span>
    ),
  },
];

const auditColumns: Column<AuditLogEntry>[] = [
  {
    key: 'timestamp',
    header: 'Time',
    render: (log) => (
      <span className="text-muted-foreground text-sm">
        {formatDistanceToNow(new Date(log.timestamp), { addSuffix: true })}
      </span>
    ),
  },
  { key: 'action', header: 'Action', render: (log) => <code className="mono-data text-primary">{log.action}</code> },
  { key: 'userId', header: 'User', render: (log) => <span className="mono-data text-sm">{log.userId}</span> },
  { key: 'resource', header: 'Resource', render: (log) => <code className="mono-data">{log.resource}</code> },
];

export default function GlobalDashboard() {
  const { data: statusData, isLoading: statusLoading } = useStatus();
  const { data: nodesData, isLoading: nodesLoading } = useNodes({ limit: 5 });
  const { data: auditData, isLoading: auditLoading } = useAuditLogs({ limit: 5 });
  const { data: jobsData, isLoading: jobsLoading } = useProvisioningJobs({ limit: 4 });

  const stats = statusData?.stats;
  const recentNodes = nodesData?.items || [];
  const recentAudit = auditData?.items || [];
  const recentJobs = jobsData?.items || [];

  const healthyPercent = stats
    ? Math.round((stats.healthyNodes / Math.max(stats.totalNodes, 1)) * 100)
    : 0;

  // Derive warning/critical counts from nodes_by_status if available
  const raw = statusData?.raw;
  const warningNodes = 0; // No 'warning' status in backend
  const criticalNodes = (raw?.nodes_by_status?.['offline'] || 0) + (raw?.nodes_by_status?.['error'] || 0);

  return (
    <AppLayout title="Platform Dashboard">
      <PageHeader
        title="Platform Overview"
        description="Global EdgeFabric infrastructure status"
        icon={Activity}
      />

      {/* Stats Grid */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4 mb-6">
        {statusLoading ? (
          <>
            <Skeleton className="h-28" />
            <Skeleton className="h-28" />
            <Skeleton className="h-28" />
            <Skeleton className="h-28" />
          </>
        ) : (
          <>
            <StatCard
              title="Total Nodes"
              value={stats?.totalNodes ?? 0}
              subtitle={`${stats?.healthyNodes ?? 0} healthy`}
              icon={Server}
              variant={healthyPercent >= 90 ? 'healthy' : healthyPercent >= 70 ? 'warning' : 'critical'}
            />
            <StatCard
              title="Active Tenants"
              value={stats?.activeTenants ?? 0}
              subtitle={`of ${stats?.totalTenants ?? 0} total`}
              icon={Building2}
            />
            <StatCard
              title="DNS Zones"
              value={stats?.totalZones ?? 0}
              icon={Globe}
            />
            <StatCard
              title="CDN Services"
              value={stats?.totalCDNServices ?? 0}
              icon={Layers}
            />
          </>
        )}
      </div>

      {/* Health & Jobs Row */}
      <div className="grid gap-6 lg:grid-cols-3 mb-6">
        {/* Fleet Health Summary */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-medium flex items-center gap-2">
              <HardDrive className="h-4 w-4 text-primary" />
              Fleet Health
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {statusLoading ? (
              <Skeleton className="h-20" />
            ) : (
              <>
                <div className="flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">Overall Health</span>
                  <span className="text-sm font-medium">{healthyPercent}%</span>
                </div>
                <Progress value={healthyPercent} className="h-2" />
                <div className="grid grid-cols-3 gap-4 pt-2">
                  <div className="text-center">
                    <div className="flex items-center justify-center gap-1 text-status-healthy">
                      <CheckCircle className="h-4 w-4" />
                      <span className="text-lg font-semibold">{stats?.healthyNodes ?? 0}</span>
                    </div>
                    <p className="text-xs text-muted-foreground">Healthy</p>
                  </div>
                  <div className="text-center">
                    <div className="flex items-center justify-center gap-1 text-status-warning">
                      <AlertTriangle className="h-4 w-4" />
                      <span className="text-lg font-semibold">{warningNodes}</span>
                    </div>
                    <p className="text-xs text-muted-foreground">Warning</p>
                  </div>
                  <div className="text-center">
                    <div className="flex items-center justify-center gap-1 text-status-critical">
                      <AlertTriangle className="h-4 w-4" />
                      <span className="text-lg font-semibold">{criticalNodes}</span>
                    </div>
                    <p className="text-xs text-muted-foreground">Critical</p>
                  </div>
                </div>
              </>
            )}
          </CardContent>
        </Card>

        {/* Provisioning Jobs */}
        <Card className="lg:col-span-2">
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-medium">Active Jobs</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {jobsLoading ? (
              <div className="space-y-3">
                <Skeleton className="h-16" />
                <Skeleton className="h-16" />
              </div>
            ) : recentJobs.length === 0 ? (
              <p className="text-sm text-muted-foreground py-4 text-center">No recent jobs</p>
            ) : (
              recentJobs.map((job) => (
                <JobRow key={job.id} job={job} />
              ))
            )}
          </CardContent>
        </Card>
      </div>

      {/* Tables Row */}
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-medium">Recent Node Activity</CardTitle>
          </CardHeader>
          <CardContent>
            {nodesLoading ? (
              <Skeleton className="h-40" />
            ) : (
              <DataTable
                data={recentNodes}
                columns={nodeColumns}
                pageSize={5}
              />
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-medium">Audit Log</CardTitle>
          </CardHeader>
          <CardContent>
            {auditLoading ? (
              <Skeleton className="h-40" />
            ) : (
              <DataTable
                data={recentAudit}
                columns={auditColumns}
                pageSize={5}
              />
            )}
          </CardContent>
        </Card>
      </div>
    </AppLayout>
  );
}

function JobRow({ job }: { job: ProvisioningJob }) {
  const statusColors = {
    pending: 'text-status-warning',
    running: 'text-status-syncing',
    completed: 'text-status-healthy',
    failed: 'text-status-critical',
  };

  return (
    <div className="flex items-center gap-4 rounded-lg border border-border p-3">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <p className="text-sm font-medium truncate">{job.targetName}</p>
          <span className={`text-xs ${statusColors[job.status]}`}>
            {job.status}
          </span>
        </div>
        <p className="text-xs text-muted-foreground">{job.type.replace(/_/g, ' ')}</p>
      </div>
      <div className="w-24">
        <Progress value={job.progress} className="h-1.5" />
      </div>
      <span className="text-xs text-muted-foreground w-10 text-right">{job.progress}%</span>
    </div>
  );
}
