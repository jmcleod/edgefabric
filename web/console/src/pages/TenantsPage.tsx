import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useTenants } from '@/hooks/useTenants';
import type { Tenant } from '@/types';
import { Building2, Eye, MoreHorizontal } from 'lucide-react';
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
  const { data, isLoading } = useTenants();
  const tenants = data?.items || [];

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
      key: 'createdAt',
      header: 'Created',
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
        description={`${data?.total ?? 0} registered tenants`}
        icon={Building2}
        action={{
          label: 'Add Tenant',
          onClick: () => console.log('Add tenant'),
        }}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={tenants}
          columns={columns}
          searchKeys={['name', 'slug']}
          pageSize={10}
          onRowClick={(tenant) => navigate(`/tenants/${tenant.id}`)}
        />
      )}
    </AppLayout>
  );
}
