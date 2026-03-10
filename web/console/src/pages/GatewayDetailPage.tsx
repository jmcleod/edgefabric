import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { AppLayout } from '@/components/layout/AppLayout';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useGateway, useUpdateGateway, useDeleteGateway } from '@/hooks/useGateways';
import { gatewaySchema, type GatewayFormData } from '@/lib/schemas';
import { Waypoints, ArrowLeft, Pencil, Trash2 } from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';

const gatewayFields: FieldConfig<GatewayFormData>[] = [
  { name: 'name', label: 'Name', placeholder: 'us-east-gw-01' },
  { name: 'tenant_id', label: 'Tenant ID', placeholder: 'Tenant UUID' },
  { name: 'public_ip', label: 'Public IP', placeholder: '203.0.113.1' },
];

export default function GatewayDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { data: gateway, isLoading, error } = useGateway(id);
  const updateGateway = useUpdateGateway();
  const deleteGateway = useDeleteGateway();

  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  if (isLoading) {
    return (
      <AppLayout>
        <Skeleton className="h-12 w-64 mb-6" />
        <Skeleton className="h-48" />
      </AppLayout>
    );
  }

  if (!gateway || error) {
    return (
      <AppLayout>
        <div className="flex flex-col items-center justify-center h-[50vh] text-center">
          <Waypoints className="h-12 w-12 text-muted-foreground mb-4" />
          <h2 className="text-xl font-semibold">Gateway not found</h2>
          <p className="text-muted-foreground mb-4">The requested gateway does not exist.</p>
          <Button onClick={() => navigate('/gateways')}>Back to Gateways</Button>
        </div>
      </AppLayout>
    );
  }

  return (
    <AppLayout
      breadcrumbs={[
        { label: 'Infrastructure' },
        { label: 'Gateways', href: '/gateways' },
        { label: gateway.name },
      ]}
    >
      <div className="mb-6">
        <Button variant="ghost" size="sm" onClick={() => navigate('/gateways')} className="mb-4 -ml-2">
          <ArrowLeft className="mr-2 h-4 w-4" /> Back to Gateways
        </Button>

        <div className="flex items-start justify-between">
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
              <Waypoints className="h-6 w-6 text-primary" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-semibold">{gateway.name}</h1>
                <StatusBadge status={gateway.status} />
              </div>
              <p className="text-muted-foreground mono-data">{gateway.publicIp}</p>
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
            <CardTitle className="text-base">Gateway Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <InfoRow label="ID" value={gateway.id} mono />
            <InfoRow label="Name" value={gateway.name} />
            <InfoRow label="Public IP" value={gateway.publicIp} mono />
            <InfoRow label="Status" value={gateway.status} />
            <InfoRow label="Last Seen" value={
              gateway.lastSeen
                ? formatDistanceToNow(new Date(gateway.lastSeen), { addSuffix: true })
                : '\u2014'
            } />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Metadata</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <InfoRow label="Hostname" value={gateway.hostname} />
            <InfoRow label="Location" value={gateway.location} />
            {gateway.tenantId && <InfoRow label="Tenant ID" value={gateway.tenantId} mono />}
          </CardContent>
        </Card>
      </div>

      {/* Edit Dialog */}
      <FormDialog<GatewayFormData>
        open={editOpen}
        onOpenChange={setEditOpen}
        title="Edit Gateway"
        schema={gatewaySchema}
        defaultValues={{ name: gateway.name, tenant_id: gateway.tenantId || '', public_ip: gateway.publicIp }}
        fields={gatewayFields}
        onSubmit={async (data) => {
          await updateGateway.mutateAsync({ id: gateway.id, body: data });
          setEditOpen(false);
        }}
        isSubmitting={updateGateway.isPending}
        submitLabel="Save Changes"
      />

      {/* Delete Confirmation */}
      <DeleteConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        entityName={gateway.name}
        onConfirm={async () => {
          await deleteGateway.mutateAsync(gateway.id);
          navigate('/gateways');
        }}
        isDeleting={deleteGateway.isPending}
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
