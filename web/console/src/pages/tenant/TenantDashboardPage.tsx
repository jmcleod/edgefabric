import { AppLayout } from '@/components/layout/AppLayout';
import { StatCard } from '@/components/ui/StatCard';
import { PageHeader } from '@/components/ui/PageHeader';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Skeleton } from '@/components/ui/skeleton';
import { useAuth } from '@/hooks/useAuth';
import { useNodes } from '@/hooks/useNodes';
import { useDNSZones } from '@/hooks/useDNS';
import { useCDNSites } from '@/hooks/useCDN';
import { useRoutes } from '@/hooks/useRoutes';
import { useAuditLogs } from '@/hooks/useAudit';
import type { Node, AuditLogEntry } from '@/types';
import { LayoutDashboard, Server, Globe, Layers, ArrowRightLeft } from 'lucide-react';
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
  { key: 'resource', header: 'Resource', render: (log) => <code className="mono-data">{log.resource}</code> },
];

export default function TenantDashboardPage() {
  const { user } = useAuth();
  const tenantId = user?.tenantId;

  const { data: nodesData, isLoading: nodesLoading } = useNodes({ limit: 5 });
  const { data: zonesData, isLoading: zonesLoading } = useDNSZones(tenantId);
  const { data: cdnData, isLoading: cdnLoading } = useCDNSites(tenantId);
  const { data: routesData, isLoading: routesLoading } = useRoutes(tenantId);
  const { data: auditData, isLoading: auditLoading } = useAuditLogs({ limit: 5 });

  const recentNodes = nodesData?.items || [];
  const recentAudit = auditData?.items || [];
  const statsLoading = zonesLoading || cdnLoading || routesLoading;

  return (
    <AppLayout title="Tenant Dashboard">
      <PageHeader
        title="Dashboard"
        description="Your EdgeFabric service overview"
        icon={LayoutDashboard}
      />

      {/* Stats Grid */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4 mb-6">
        {statsLoading ? (
          <>
            <Skeleton className="h-28" />
            <Skeleton className="h-28" />
            <Skeleton className="h-28" />
            <Skeleton className="h-28" />
          </>
        ) : (
          <>
            <StatCard
              title="Assigned Nodes"
              value={nodesData?.total ?? 0}
              icon={Server}
            />
            <StatCard
              title="DNS Zones"
              value={zonesData?.total ?? 0}
              icon={Globe}
            />
            <StatCard
              title="CDN Services"
              value={cdnData?.total ?? 0}
              icon={Layers}
            />
            <StatCard
              title="Routes"
              value={routesData?.total ?? 0}
              icon={ArrowRightLeft}
            />
          </>
        )}
      </div>

      {/* Tables Row */}
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-medium">Assigned Nodes</CardTitle>
          </CardHeader>
          <CardContent>
            {nodesLoading ? (
              <Skeleton className="h-40" />
            ) : (
              <DataTable data={recentNodes} columns={nodeColumns} pageSize={5} />
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-medium">Recent Activity</CardTitle>
          </CardHeader>
          <CardContent>
            {auditLoading ? (
              <Skeleton className="h-40" />
            ) : (
              <DataTable data={recentAudit} columns={auditColumns} pageSize={5} />
            )}
          </CardContent>
        </Card>
      </div>
    </AppLayout>
  );
}
