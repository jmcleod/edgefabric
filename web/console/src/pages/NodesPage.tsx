import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { nodes } from '@/data/mockData';
import type { Node } from '@/types';
import { Server, Eye, MoreHorizontal } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

export default function NodesPage() {
  const navigate = useNavigate();

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
    { key: 'location', header: 'Location' },
    { key: 'region', header: 'Region', className: 'hidden lg:table-cell' },
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
      key: 'metrics',
      header: 'CPU / Mem',
      className: 'hidden lg:table-cell',
      render: (node) => (
        <div className="text-sm tabular-nums">
          <span className={node.cpu > 80 ? 'text-status-warning font-medium' : ''}>{node.cpu}%</span>
          {' / '}
          <span className={node.memory > 80 ? 'text-status-warning font-medium' : ''}>{node.memory}%</span>
        </div>
      ),
    },
    {
      key: 'uptime',
      header: 'Uptime',
      className: 'hidden xl:table-cell',
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
        description={`${nodes.length} nodes across all regions`}
        icon={Server}
        action={{
          label: 'Add Node',
          onClick: () => console.log('Add node'),
        }}
      />

      <DataTable
        data={nodes}
        columns={columns}
        searchKeys={['name', 'hostname', 'ipv4', 'location', 'region']}
        pageSize={10}
        onRowClick={(node) => navigate(`/nodes/${node.id}`)}
      />
    </AppLayout>
  );
}
