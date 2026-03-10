import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useTenants, useCreateTenant, useUpdateTenant, useDeleteTenant } from '@/hooks/useTenants';
import { tenantSchema, type TenantFormData } from '@/lib/schemas';
import type { Tenant } from '@/types';
import { Building2, Eye, MoreHorizontal, Pencil, Trash2 } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

const tenantFields: FieldConfig<TenantFormData>[] = [
  { name: 'name', label: 'Name', placeholder: 'Acme Corp' },
  { name: 'slug', label: 'Slug', placeholder: 'acme-corp', description: 'URL-safe identifier (lowercase, hyphens only)' },
  { name: 'status', label: 'Status', type: 'select', options: [
    { label: 'Active', value: 'active' },
    { label: 'Suspended', value: 'suspended' },
  ] },
];

export default function TenantsPage() {
  const navigate = useNavigate();
  const { data, isLoading } = useTenants();
  const tenants = data?.items || [];
  const createTenant = useCreateTenant();
  const updateTenant = useUpdateTenant();
  const deleteTenant = useDeleteTenant();

  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Tenant | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Tenant | null>(null);

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
            <DropdownMenuItem onClick={() => setEditTarget(tenant)}>
              <Pencil className="mr-2 h-4 w-4" /> Edit Tenant
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(tenant)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete Tenant
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
          onClick: () => setCreateOpen(true),
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

      {/* Create Dialog */}
      <FormDialog<TenantFormData>
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Create Tenant"
        description="Add a new tenant to the platform."
        schema={tenantSchema}
        defaultValues={{ name: '', slug: '', status: 'active' }}
        fields={tenantFields}
        onSubmit={async (data) => {
          await createTenant.mutateAsync(data);
          setCreateOpen(false);
        }}
        isSubmitting={createTenant.isPending}
        submitLabel="Create Tenant"
      />

      {/* Edit Dialog */}
      <FormDialog<TenantFormData>
        open={!!editTarget}
        onOpenChange={(open) => !open && setEditTarget(null)}
        title="Edit Tenant"
        schema={tenantSchema}
        defaultValues={editTarget ? {
          name: editTarget.name,
          slug: editTarget.slug,
          status: editTarget.status === 'pending' ? 'active' : editTarget.status,
        } : undefined}
        fields={tenantFields}
        onSubmit={async (data) => {
          if (editTarget) {
            await updateTenant.mutateAsync({ id: editTarget.id, body: data });
            setEditTarget(null);
          }
        }}
        isSubmitting={updateTenant.isPending}
        submitLabel="Save Changes"
      />

      {/* Delete Confirmation */}
      <DeleteConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        entityName={deleteTarget?.name}
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteTenant.mutateAsync(deleteTarget.id);
            setDeleteTarget(null);
          }
        }}
        isDeleting={deleteTenant.isPending}
      />
    </AppLayout>
  );
}
