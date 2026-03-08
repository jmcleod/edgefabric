import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { auditLogs } from '@/data/mockData';
import type { AuditLogEntry } from '@/types';
import { FileText } from 'lucide-react';
import { formatDistanceToNow, format } from 'date-fns';

export default function AuditLogsPage() {
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
      key: 'userEmail',
      header: 'User',
      render: (log) => (
        <div>
          <p className="text-sm">{log.userEmail}</p>
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
          <code className="text-xs text-muted-foreground mono-data">{log.resourceId}</code>
        </div>
      ),
    },
    {
      key: 'details',
      header: 'Details',
      className: 'hidden xl:table-cell max-w-xs',
      render: (log) => (
        <code className="text-xs text-muted-foreground mono-data truncate block">
          {log.details ? JSON.stringify(log.details) : '-'}
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

      <DataTable
        data={auditLogs}
        columns={columns}
        searchKeys={['action', 'userEmail', 'resource', 'resourceId']}
        pageSize={15}
      />
    </AppLayout>
  );
}
