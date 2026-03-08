import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useGateways } from '@/hooks/useGateways';
import type { Gateway } from '@/types';
import { Waypoints, Eye, MoreHorizontal } from 'lucide-react';
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
  const { data, isLoading } = useGateways();
  const gateways = data?.items || [];

  const columns: Column<Gateway>[] = [
    {
      key: 'name',
      header: 'Gateway',
      render: (gw) => (
        <div>
          <p className="font-medium text-foreground">{gw.name}</p>
          <p className="text-xs text-muted-foreground mono-data">{gw.publicIp}</p>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (gw) => <StatusBadge status={gw.status} size="sm" />,
    },
    {
      key: 'lastSeen',
      header: 'Last Seen',
      render: (gw) => (
        <span className="text-muted-foreground text-sm">
          {gw.lastSeen
            ? formatDistanceToNow(new Date(gw.lastSeen), { addSuffix: true })
            : '\u2014'}
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

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={gateways}
          columns={columns}
          searchKeys={['name', 'publicIp']}
          pageSize={10}
          onRowClick={(gw) => navigate(`/gateways/${gw.id}`)}
        />
      )}
    </AppLayout>
  );
}
