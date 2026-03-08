import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { tenants } from '@/data/mockData';
import type { Tenant } from '@/types';
import { Building2, Eye, MoreHorizontal, Server, Globe, Layers } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

export default function TenantsPage() {
  const navigate = useNavigate();

  const columns: Column<Tenant>[] = [
    {
      key: 'name',
      header: 'Tenant',
      render: (tenant) => (
        <div>
          <p className="font-medium text-foreground">{tenant.name}</p>
          <p className="text-xs text-muted-foreground mono-data">{tenant.slug}</p>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (tenant) => <StatusBadge status={tenant.status} size="sm" />,
    },
    {
      key: 'nodeCount',
      header: 'Nodes',
      render: (tenant) => (
        <div className="flex items-center gap-1.5">
          <Server className="h-3.5 w-3.5 text-muted-foreground" />
          <span>{tenant.nodeCount}</span>
        </div>
      ),
    },
    {
      key: 'zoneCount',
      header: 'DNS Zones',
      render: (tenant) => (
        <div className="flex items-center gap-1.5">
          <Globe className="h-3.5 w-3.5 text-muted-foreground" />
          <span>{tenant.zoneCount}</span>
        </div>
      ),
    },
    {
      key: 'cdnServiceCount',
      header: 'CDN Services',
      render: (tenant) => (
        <div className="flex items-center gap-1.5">
          <Layers className="h-3.5 w-3.5 text-muted-foreground" />
          <span>{tenant.cdnServiceCount}</span>
        </div>
      ),
    },
    {
      key: 'createdAt',
      header: 'Created',
      className: 'hidden lg:table-cell',
      render: (tenant) => new Date(tenant.createdAt).toLocaleDateString(),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (tenant) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => navigate(`/tenants/${tenant.id}`)}>
              <Eye className="mr-2 h-4 w-4" /> View Details
            </DropdownMenuItem>
            <DropdownMenuItem>Edit Tenant</DropdownMenuItem>
            <DropdownMenuItem>Manage Users</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive">
              {tenant.status === 'active' ? 'Suspend Tenant' : 'Reactivate Tenant'}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Infrastructure' }, { label: 'Tenants' }]}>
      <PageHeader
        title="Tenants"
        description={`${tenants.length} registered tenants`}
        icon={Building2}
        action={{
          label: 'Add Tenant',
          onClick: () => console.log('Add tenant'),
        }}
      />

      <DataTable
        data={tenants}
        columns={columns}
        searchKeys={['name', 'slug']}
        pageSize={10}
        onRowClick={(tenant) => navigate(`/tenants/${tenant.id}`)}
      />
    </AppLayout>
  );
}
