import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { AppLayout } from '@/components/layout/AppLayout';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useTenant, useTenantStats, useUpdateTenant, useDeleteTenant } from '@/hooks/useTenants';
import { tenantSchema, type TenantFormData } from '@/lib/schemas';
import { Building2, ArrowLeft, Pencil, Trash2 } from 'lucide-react';

const tenantFields: FieldConfig<TenantFormData>[] = [
  { name: 'name', label: 'Name', placeholder: 'Acme Corp' },
  { name: 'slug', label: 'Slug', placeholder: 'acme-corp', description: 'URL-safe identifier' },
  { name: 'status', label: 'Status', type: 'select', options: [
    { label: 'Active', value: 'active' },
    { label: 'Suspended', value: 'suspended' },
  ] },
];

export default function TenantDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { data: tenant, isLoading, error } = useTenant(id);
  const { data: stats } = useTenantStats(id);
  const updateTenant = useUpdateTenant();
  const deleteTenant = useDeleteTenant();

  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  if (isLoading) {
    return (
      <AppLayout>
        <Skeleton className="h-12 w-64 mb-6" />
        <div className="grid gap-4 md:grid-cols-2 mb-6">
          <Skeleton className="h-48" />
          <Skeleton className="h-48" />
        </div>
      </AppLayout>
    );
  }

  if (!tenant || error) {
    return (
      <AppLayout>
        <div className="flex flex-col items-center justify-center h-[50vh] text-center">
          <Building2 className="h-12 w-12 text-muted-foreground mb-4" />
          <h2 className="text-xl font-semibold">Tenant not found</h2>
          <p className="text-muted-foreground mb-4">The requested tenant does not exist.</p>
          <Button onClick={() => navigate('/tenants')}>Back to Tenants</Button>
        </div>
      </AppLayout>
    );
  }

  return (
    <AppLayout
      breadcrumbs={[
        { label: 'Infrastructure' },
        { label: 'Tenants', href: '/tenants' },
        { label: tenant.name },
      ]}
    >
      <div className="mb-6">
        <Button variant="ghost" size="sm" onClick={() => navigate('/tenants')} className="mb-4 -ml-2">
          <ArrowLeft className="mr-2 h-4 w-4" /> Back to Tenants
        </Button>

        <div className="flex items-start justify-between">
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
              <Building2 className="h-6 w-6 text-primary" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-semibold">{tenant.name}</h1>
                <StatusBadge status={tenant.status} />
              </div>
              <p className="text-muted-foreground mono-data">{tenant.slug}</p>
            </div>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
              <Pencil className="mr-2 h-4 w-4" /> Edit
            </Button>
            <Button variant="destructive" size="sm" onClick={() => setDeleteOpen(true)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete
            </Button>
          </div>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Tenant Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <InfoRow label="ID" value={tenant.id} mono />
            <InfoRow label="Name" value={tenant.name} />
            <InfoRow label="Slug" value={tenant.slug} mono />
            <InfoRow label="Status" value={tenant.status} />
            <InfoRow label="Created" value={new Date(tenant.createdAt).toLocaleDateString()} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Resource Summary</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <InfoRow label="Assigned Nodes" value={String(stats?.nodeCount ?? tenant.nodeCount)} />
            <InfoRow label="DNS Zones" value={String(stats?.zoneCount ?? tenant.zoneCount)} />
            <InfoRow label="CDN Services" value={String(stats?.cdnServiceCount ?? tenant.cdnServiceCount)} />
          </CardContent>
        </Card>
      </div>

      {/* Edit Dialog */}
      <FormDialog<TenantFormData>
        open={editOpen}
        onOpenChange={setEditOpen}
        title="Edit Tenant"
        schema={tenantSchema}
        defaultValues={{ name: tenant.name, slug: tenant.slug, status: tenant.status === 'pending' ? 'active' : tenant.status }}
        fields={tenantFields}
        onSubmit={async (data) => {
          await updateTenant.mutateAsync({ id: tenant.id, body: data });
          setEditOpen(false);
        }}
        isSubmitting={updateTenant.isPending}
        submitLabel="Save Changes"
      />

      {/* Delete Confirmation */}
      <DeleteConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        entityName={tenant.name}
        onConfirm={async () => {
          await deleteTenant.mutateAsync(tenant.id);
          navigate('/tenants');
        }}
        isDeleting={deleteTenant.isPending}
      />
    </AppLayout>
  );
}

function InfoRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex justify-between items-center py-1 border-b border-border/50 last:border-0">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className={`text-sm ${mono ? 'mono-data' : ''}`}>{value}</span>
    </div>
  );
}
