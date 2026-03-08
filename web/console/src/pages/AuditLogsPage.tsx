import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { Skeleton } from '@/components/ui/skeleton';
import { useAuditLogs } from '@/hooks/useAudit';
import type { AuditLogEntry } from '@/types';
import { FileText } from 'lucide-react';
import { formatDistanceToNow, format } from 'date-fns';

export default function AuditLogsPage() {
  const { data, isLoading } = useAuditLogs();
  const auditLogs = data?.items || [];

  const columns: Column<AuditLogEntry>[] = [
    {
      key: 'timestamp',
      header: 'Time',
      render: (log) => (
        <div>
          <p className="text-sm">{format(new Date(log.timestamp), 'MMM d, HH:mm:ss')}</p>
          <p className="text-xs text-muted-foreground">
            {formatDistanceToNow(new Date(log.timestamp), { addSuffix: true })}
          </p>
        </div>
      ),
    },
    {
      key: 'action',
      header: 'Action',
      render: (log) => <code className="mono-data text-primary">{log.action}</code>,
    },
    {
      key: 'userId',
      header: 'User',
      render: (log) => (
        <div>
          <p className="text-sm mono-data">{log.userId}</p>
          <p className="text-xs text-muted-foreground mono-data">{log.ipAddress}</p>
        </div>
      ),
    },
    {
      key: 'resource',
      header: 'Resource',
      render: (log) => (
        <div>
          <p className="text-sm">{log.resource}</p>
          {log.resourceId && <code className="text-xs text-muted-foreground mono-data">{log.resourceId}</code>}
        </div>
      ),
    },
    {
      key: 'details',
      header: 'Details',
      className: 'hidden xl:table-cell max-w-xs',
      render: (log) => (
        <code className="text-xs text-muted-foreground mono-data truncate block">
          {log.details ? JSON.stringify(log.details) : '\u2014'}
        </code>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Operations' }, { label: 'Audit Logs' }]}>
      <PageHeader
        title="Audit Logs"
        description="Platform-wide activity history"
        icon={FileText}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={auditLogs}
          columns={columns}
          searchKeys={['action', 'userId', 'resource']}
          pageSize={15}
        />
      )}
    </AppLayout>
  );
}
