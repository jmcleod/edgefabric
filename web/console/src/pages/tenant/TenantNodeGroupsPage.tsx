import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { Skeleton } from '@/components/ui/skeleton';
import { useNodeGroups } from '@/hooks/useNodeGroups';
import type { NodeGroup } from '@/types';
import { Boxes } from 'lucide-react';

export default function TenantNodeGroupsPage() {
  const { data, isLoading } = useNodeGroups();
  const groups = data?.items || [];

  const columns: Column<NodeGroup>[] = [
    {
      key: 'name',
      header: 'Name',
      render: (g) => (
        <div>
          <p className="font-medium text-foreground">{g.name}</p>
          {g.description && <p className="text-xs text-muted-foreground">{g.description}</p>}
        </div>
      ),
    },
    {
      key: 'nodeCount',
      header: 'Nodes',
      render: (g) => <span className="text-sm">{g.nodeCount}</span>,
    },
    {
      key: 'createdAt',
      header: 'Created',
      render: (g) => <span className="text-sm text-muted-foreground">{new Date(g.createdAt).toLocaleDateString()}</span>,
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Infrastructure' }, { label: 'Node Groups' }]}>
      <PageHeader
        title="Node Groups"
        description={`${data?.total ?? 0} groups`}
        icon={Boxes}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={groups}
          columns={columns}
          searchKeys={['name']}
          pageSize={10}
          emptyMessage="No node groups available"
        />
      )}
    </AppLayout>
  );
}
