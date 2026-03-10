import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { StatCard } from '@/components/ui/StatCard';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { useStatus } from '@/hooks/useStatus';
import { useNodes } from '@/hooks/useNodes';
import { useAuditLogs } from '@/hooks/useAudit';
import type { Node, AuditLogEntry } from '@/types';
import {
  Activity,
  Server,
  CheckCircle,
  AlertTriangle,
  XCircle,
  Shield,
  Radio,
} from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';

export default function FleetHealthPage() {
  const { data: statusData, isLoading: statusLoading } = useStatus();
  const { data: nodesData, isLoading: nodesLoading } = useNodes();
  const { data: auditData, isLoading: auditLoading } = useAuditLogs({ limit: 10 });

  const stats = statusData?.stats;
  const raw = statusData?.raw;
  const nodes = nodesData?.items || [];
  const recentAudit = auditData?.items || [];

  const healthyNodes = stats?.healthyNodes ?? 0;
  const totalNodes = stats?.totalNodes ?? 1;
  const healthyPercent = Math.round((healthyNodes / Math.max(totalNodes, 1)) * 100);

  // Derive status counts
  const onlineCount = raw?.nodes_by_status?.['online'] || 0;
  const offlineCount = raw?.nodes_by_status?.['offline'] || 0;
  const errorCount = raw?.nodes_by_status?.['error'] || 0;

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
      key: 'version',
      header: 'Version',
      render: (node) => <code className="mono-data text-xs">{node.version}</code>,
    },
    {
      key: 'cpu',
      header: 'CPU',
      render: (node) => (
        <div className="flex items-center gap-2 w-24">
          <Progress value={node.cpu} className="h-1.5 flex-1" />
          <span className="text-xs text-muted-foreground w-8 text-right">{node.cpu}%</span>
        </div>
      ),
    },
    {
      key: 'memory',
      header: 'Memory',
      render: (node) => (
        <div className="flex items-center gap-2 w-24">
          <Progress value={node.memory} className="h-1.5 flex-1" />
          <span className="text-xs text-muted-foreground w-8 text-right">{node.memory}%</span>
        </div>
      ),
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
    {
      key: 'action',
      header: 'Action',
      render: (log) => <code className="mono-data text-primary">{log.action}</code>,
    },
    {
      key: 'resource',
      header: 'Resource',
      render: (log) => <code className="mono-data">{log.resource}</code>,
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Overview' }, { label: 'Fleet Health' }]}>
      <PageHeader
        title="Fleet Health"
        description="Aggregate infrastructure health overview"
        icon={Activity}
      />

      {/* Health Stats */}
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
              title="Fleet Health"
              value={`${healthyPercent}%`}
              subtitle={`${healthyNodes} of ${totalNodes} nodes healthy`}
              icon={Server}
              variant={healthyPercent >= 90 ? 'healthy' : healthyPercent >= 70 ? 'warning' : 'critical'}
            />
            <StatCard
              title="Online"
              value={onlineCount}
              icon={CheckCircle}
              variant="healthy"
            />
            <StatCard
              title="Offline"
              value={offlineCount}
              icon={AlertTriangle}
              variant={offlineCount > 0 ? 'warning' : 'healthy'}
            />
            <StatCard
              title="Error"
              value={errorCount}
              icon={XCircle}
              variant={errorCount > 0 ? 'critical' : 'healthy'}
            />
          </>
        )}
      </div>

      {/* Node List */}
      <Card className="mb-6">
        <CardHeader className="pb-3">
          <CardTitle className="text-base font-medium">All Nodes</CardTitle>
        </CardHeader>
        <CardContent>
          {nodesLoading ? (
            <Skeleton className="h-64" />
          ) : (
            <DataTable
              data={nodes}
              columns={nodeColumns}
              searchKeys={['name', 'region', 'ipv4']}
              pageSize={10}
            />
          )}
        </CardContent>
      </Card>

      {/* Recent Events */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base font-medium">Recent Events</CardTitle>
        </CardHeader>
        <CardContent>
          {auditLoading ? (
            <Skeleton className="h-40" />
          ) : (
            <DataTable data={recentAudit} columns={auditColumns} pageSize={10} />
          )}
        </CardContent>
      </Card>
    </AppLayout>
  );
}
