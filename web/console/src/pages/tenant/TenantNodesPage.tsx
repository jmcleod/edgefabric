import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Skeleton } from '@/components/ui/skeleton';
import { useNodes } from '@/hooks/useNodes';
import type { Node } from '@/types';
import { Server } from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';

export default function TenantNodesPage() {
  const { data, isLoading } = useNodes();
  const nodes = data?.items || [];

  const columns: Column<Node>[] = [
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
    {
      key: 'region',
      header: 'Region',
      render: (node) => <span className="text-sm">{node.region}</span>,
    },
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

  return (
    <AppLayout breadcrumbs={[{ label: 'Infrastructure' }, { label: 'Assigned Nodes' }]}>
      <PageHeader
        title="Assigned Nodes"
        description={`${data?.total ?? 0} nodes assigned to your tenant`}
        icon={Server}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={nodes}
          columns={columns}
          searchKeys={['name', 'region', 'ipv4']}
          pageSize={10}
          emptyMessage="No nodes assigned to your tenant"
        />
      )}
    </AppLayout>
  );
}
