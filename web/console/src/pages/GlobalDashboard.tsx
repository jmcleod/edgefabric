import { AppLayout } from '@/components/layout/AppLayout';
import { StatCard } from '@/components/ui/StatCard';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { DataTable, Column } from '@/components/ui/DataTable';
import { PageHeader } from '@/components/ui/PageHeader';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { globalStats, nodes, provisioningJobs, auditLogs } from '@/data/mockData';
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

const recentNodes = nodes.slice(0, 5);
const recentJobs = provisioningJobs.slice(0, 4);
const recentAudit = auditLogs.slice(0, 5);

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
  { key: 'location', header: 'Location' },
  {
    key: 'status',
    header: 'Status',
    render: (node) => <StatusBadge status={node.status} size="sm" />,
  },
  {
    key: 'metrics',
    header: 'CPU / Mem',
    render: (node) => (
      <div className="text-sm">
        <span className={node.cpu > 80 ? 'text-status-warning' : ''}>{node.cpu}%</span>
        {' / '}
        <span className={node.memory > 80 ? 'text-status-warning' : ''}>{node.memory}%</span>
      </div>
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
  { key: 'userEmail', header: 'User' },
  { key: 'resourceId', header: 'Resource', render: (log) => <code className="mono-data">{log.resourceId}</code> },
];

export default function GlobalDashboard() {
  const healthyPercent = Math.round((globalStats.healthyNodes / globalStats.totalNodes) * 100);
  const criticalNodes = nodes.filter((n) => n.status === 'critical').length;
  const warningNodes = nodes.filter((n) => n.status === 'warning').length;

  return (
    <AppLayout title="Platform Dashboard">
      <PageHeader
        title="Platform Overview"
        description="Global EdgeFabric infrastructure status"
        icon={Activity}
      />

      {/* Stats Grid */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4 mb-6">
        <StatCard
          title="Total Nodes"
          value={globalStats.totalNodes}
          subtitle={`${globalStats.healthyNodes} healthy`}
          icon={Server}
          variant={healthyPercent >= 90 ? 'healthy' : healthyPercent >= 70 ? 'warning' : 'critical'}
        />
        <StatCard
          title="Active Tenants"
          value={globalStats.activeTenants}
          subtitle={`of ${globalStats.totalTenants} total`}
          icon={Building2}
        />
        <StatCard
          title="DNS Zones"
          value={globalStats.totalZones}
          icon={Globe}
        />
        <StatCard
          title="CDN Services"
          value={globalStats.totalCDNServices}
          subtitle={`${globalStats.bandwidthTbMonth} TB/mo`}
          icon={Layers}
        />
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
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">Overall Health</span>
              <span className="text-sm font-medium">{healthyPercent}%</span>
            </div>
            <Progress value={healthyPercent} className="h-2" />
            <div className="grid grid-cols-3 gap-4 pt-2">
              <div className="text-center">
                <div className="flex items-center justify-center gap-1 text-status-healthy">
                  <CheckCircle className="h-4 w-4" />
                  <span className="text-lg font-semibold">{globalStats.healthyNodes}</span>
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
          </CardContent>
        </Card>

        {/* Provisioning Jobs */}
        <Card className="lg:col-span-2">
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-medium">Active Jobs</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {recentJobs.map((job) => (
              <JobRow key={job.id} job={job} />
            ))}
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
            <DataTable
              data={recentNodes}
              columns={nodeColumns}
              pageSize={5}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-medium">Audit Log</CardTitle>
          </CardHeader>
          <CardContent>
            <DataTable
              data={recentAudit}
              columns={auditColumns}
              pageSize={5}
            />
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
