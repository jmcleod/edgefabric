import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useCDNSites } from '@/hooks/useCDN';
import type { CDNService } from '@/types';
import { Layers, Eye, MoreHorizontal, Globe, Trash2 } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

export default function CDNServicesPage() {
  const navigate = useNavigate();
  const { data, isLoading } = useCDNSites();
  const cdnServices = data?.items || [];

  const columns: Column<CDNService>[] = [
    {
      key: 'name',
      header: 'Service',
      render: (service) => (
        <div>
          <p className="font-medium text-foreground">{service.name}</p>
          <p className="text-xs text-muted-foreground mono-data">{service.id}</p>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (service) => <StatusBadge status={service.status} size="sm" />,
    },
    {
      key: 'domainCount',
      header: 'Domains',
      render: (service) => (
        <div className="flex items-center gap-1.5">
          <Globe className="h-3.5 w-3.5 text-muted-foreground" />
          <span>{service.domainCount}</span>
        </div>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (service) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => navigate(`/tenant/cdn/services/${service.id}`)}>
              <Eye className="mr-2 h-4 w-4" /> View Details
            </DropdownMenuItem>
            <DropdownMenuItem>Edit Service</DropdownMenuItem>
            <DropdownMenuItem>
              <Trash2 className="mr-2 h-4 w-4" /> Purge Cache
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive">Delete Service</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'CDN' }, { label: 'Services' }]}>
      <PageHeader
        title="CDN Services"
        description="Content delivery and edge caching"
        icon={Layers}
        action={{
          label: 'Create Service',
          onClick: () => console.log('Create service'),
        }}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={cdnServices}
          columns={columns}
          searchKeys={['name']}
          pageSize={10}
          onRowClick={(service) => navigate(`/tenant/cdn/services/${service.id}`)}
        />
      )}
    </AppLayout>
  );
}
