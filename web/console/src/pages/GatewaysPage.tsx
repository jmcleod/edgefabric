import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { gateways } from '@/data/mockData';
import type { Gateway } from '@/types';
import { Waypoints, Eye, MoreHorizontal, Server } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { formatDistanceToNow } from 'date-fns';

export default function GatewaysPage() {
  const navigate = useNavigate();

  const columns: Column<Gateway>[] = [
    {
      key: 'name',
      header: 'Gateway',
      render: (gw) => (
        <div>
          <p className="font-medium text-foreground">{gw.name}</p>
          <p className="text-xs text-muted-foreground">{gw.hostname}</p>
        </div>
      ),
    },
    {
      key: 'publicIp',
      header: 'Public IP',
      render: (gw) => <code className="mono-data text-sm">{gw.publicIp}</code>,
    },
    { key: 'location', header: 'Location' },
    {
      key: 'status',
      header: 'Status',
      render: (gw) => <StatusBadge status={gw.status} size="sm" />,
    },
    {
      key: 'connectedNodes',
      header: 'Connected Nodes',
      render: (gw) => (
        <div className="flex items-center gap-1.5">
          <Server className="h-3.5 w-3.5 text-muted-foreground" />
          <span>{gw.connectedNodes}</span>
        </div>
      ),
    },
    {
      key: 'lastSeen',
      header: 'Last Seen',
      className: 'hidden lg:table-cell',
      render: (gw) => (
        <span className="text-muted-foreground text-sm">
          {formatDistanceToNow(new Date(gw.lastSeen), { addSuffix: true })}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (gw) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => navigate(`/gateways/${gw.id}`)}>
              <Eye className="mr-2 h-4 w-4" /> View Details
            </DropdownMenuItem>
            <DropdownMenuItem>Edit Gateway</DropdownMenuItem>
            <DropdownMenuItem>Restart Gateway</DropdownMenuItem>
            <DropdownMenuItem className="text-destructive">Remove Gateway</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Infrastructure' }, { label: 'Gateways' }]}>
      <PageHeader
        title="Gateways"
        description="WireGuard gateway nodes for private routing"
        icon={Waypoints}
        action={{
          label: 'Add Gateway',
          onClick: () => console.log('Add gateway'),
        }}
      />

      <DataTable
        data={gateways}
        columns={columns}
        searchKeys={['name', 'hostname', 'publicIp', 'location']}
        pageSize={10}
        onRowClick={(gw) => navigate(`/gateways/${gw.id}`)}
      />
    </AppLayout>
  );
}
