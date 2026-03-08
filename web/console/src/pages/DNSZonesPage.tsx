import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useDNSZones } from '@/hooks/useDNS';
import type { DNSZone } from '@/types';
import { Globe, Eye, MoreHorizontal } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { formatDistanceToNow } from 'date-fns';

export default function DNSZonesPage() {
  const navigate = useNavigate();
  const { data, isLoading } = useDNSZones();
  const dnsZones = data?.items || [];

  const columns: Column<DNSZone>[] = [
    {
      key: 'name',
      header: 'Zone',
      render: (zone) => (
        <div>
          <p className="font-medium text-foreground">{zone.name}</p>
          <p className="text-xs text-muted-foreground mono-data">Serial: {zone.serial}</p>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (zone) => <StatusBadge status={zone.status} size="sm" />,
    },
    {
      key: 'lastModified',
      header: 'Last Modified',
      render: (zone) => (
        <span className="text-muted-foreground text-sm">
          {formatDistanceToNow(new Date(zone.lastModified), { addSuffix: true })}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (zone) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => navigate(`/tenant/dns/zones/${zone.id}`)}>
              <Eye className="mr-2 h-4 w-4" /> View Records
            </DropdownMenuItem>
            <DropdownMenuItem>Edit Zone</DropdownMenuItem>
            <DropdownMenuItem>Export Zone File</DropdownMenuItem>
            <DropdownMenuItem className="text-destructive">Delete Zone</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'DNS' }, { label: 'Zones' }]}>
      <PageHeader
        title="DNS Zones"
        description="Manage authoritative DNS zones"
        icon={Globe}
        action={{
          label: 'Add Zone',
          onClick: () => console.log('Add zone'),
        }}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={dnsZones}
          columns={columns}
          searchKeys={['name']}
          pageSize={10}
          onRowClick={(zone) => navigate(`/tenant/dns/zones/${zone.id}`)}
        />
      )}
    </AppLayout>
  );
}
