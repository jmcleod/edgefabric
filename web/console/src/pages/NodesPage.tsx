import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useNodes } from '@/hooks/useNodes';
import type { Node } from '@/types';
import { Server, Eye, MoreHorizontal } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

export default function NodesPage() {
  const navigate = useNavigate();
  const { data, isLoading } = useNodes();
  const nodes = data?.items || [];

  const columns: Column<Node>[] = [
    {
      key: 'name',
      header: 'Name',
      render: (node) => (
        <div>
          <p className="font-medium text-foreground">{node.name}</p>
          <p className="text-xs text-muted-foreground">{node.hostname}</p>
        </div>
      ),
    },
    {
      key: 'ipv4',
      header: 'IP Address',
      render: (node) => (
        <code className="mono-data text-sm">{node.ipv4}</code>
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
      className: 'hidden md:table-cell',
      render: (node) => <code className="mono-data text-xs">{node.version}</code>,
    },
    {
      key: 'lastSeen',
      header: 'Last Seen',
      className: 'hidden lg:table-cell',
      render: (node) => (
        <span className="text-muted-foreground text-sm">
          {node.lastSeen
            ? formatDistanceToNow(new Date(node.lastSeen), { addSuffix: true })
            : '\u2014'}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (node) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => navigate(`/nodes/${node.id}`)}>
              <Eye className="mr-2 h-4 w-4" /> View Details
            </DropdownMenuItem>
            <DropdownMenuItem>SSH Console</DropdownMenuItem>
            <DropdownMenuItem>Restart Node</DropdownMenuItem>
            <DropdownMenuItem className="text-destructive">Remove Node</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Infrastructure' }, { label: 'Nodes' }]}>
      <PageHeader
        title="Nodes"
        description={`${data?.total ?? 0} nodes across all regions`}
        icon={Server}
        action={{
          label: 'Add Node',
          onClick: () => console.log('Add node'),
        }}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={nodes}
          columns={columns}
          searchKeys={['name', 'hostname', 'ipv4', 'region']}
          pageSize={10}
          onRowClick={(node) => navigate(`/nodes/${node.id}`)}
        />
      )}
    </AppLayout>
  );
}
