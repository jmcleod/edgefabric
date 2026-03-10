import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { AppLayout } from '@/components/layout/AppLayout';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useRoute, useUpdateRoute, useDeleteRoute } from '@/hooks/useRoutes';
import { routeSchema, type RouteFormData } from '@/lib/schemas';
import { ArrowRightLeft, ArrowLeft, Pencil, Trash2 } from 'lucide-react';

const routeFields: FieldConfig<RouteFormData>[] = [
  { name: 'name', label: 'Name', placeholder: 'web-proxy-route' },
  {
    name: 'protocol',
    label: 'Protocol',
    type: 'select',
    options: [
      { label: 'TCP', value: 'tcp' },
      { label: 'UDP', value: 'udp' },
      { label: 'ICMP', value: 'icmp' },
      { label: 'All', value: 'all' },
    ],
  },
  { name: 'entry_ip', label: 'Entry IP', placeholder: '203.0.113.1' },
  { name: 'entry_port', label: 'Entry Port', type: 'number', placeholder: '443' },
  { name: 'gateway_id', label: 'Gateway ID', placeholder: 'Gateway UUID' },
  { name: 'destination_ip', label: 'Destination IP', placeholder: '10.0.1.5' },
  { name: 'destination_port', label: 'Destination Port', type: 'number', placeholder: '8443' },
  { name: 'node_group_id', label: 'Node Group ID', placeholder: 'Optional' },
];

export default function RouteDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { data: route, isLoading, error } = useRoute(id);
  const updateRoute = useUpdateRoute();
  const deleteRoute = useDeleteRoute();

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

  if (!route || error) {
    return (
      <AppLayout>
        <div className="flex flex-col items-center justify-center h-[50vh] text-center">
          <ArrowRightLeft className="h-12 w-12 text-muted-foreground mb-4" />
          <h2 className="text-xl font-semibold">Route not found</h2>
          <p className="text-muted-foreground mb-4">The requested route does not exist.</p>
          <Button onClick={() => navigate('/tenant/routes')}>Back to Routes</Button>
        </div>
      </AppLayout>
    );
  }

  return (
    <AppLayout
      breadcrumbs={[
        { label: 'Networking' },
        { label: 'Routes', href: '/tenant/routes' },
        { label: route.name },
      ]}
    >
      <div className="mb-6">
        <Button variant="ghost" size="sm" onClick={() => navigate('/tenant/routes')} className="mb-4 -ml-2">
          <ArrowLeft className="mr-2 h-4 w-4" /> Back to Routes
        </Button>

        <div className="flex items-start justify-between">
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
              <ArrowRightLeft className="h-6 w-6 text-primary" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-semibold">{route.name}</h1>
                <StatusBadge status={route.status} />
              </div>
              <p className="text-muted-foreground text-sm mono-data">
                {route.exposedIp} &rarr; {route.privateDestination}
              </p>
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
            <CardTitle className="text-base">Route Configuration</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <InfoRow label="ID" value={route.id} mono />
            <InfoRow label="Name" value={route.name} />
            <InfoRow label="Entry IP" value={route.exposedIp} mono />
            <InfoRow label="Destination" value={route.privateDestination} mono />
            <InfoRow label="Gateway ID" value={route.gatewayId} mono />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Metadata</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <InfoRow label="Status" value={route.status} />
            <InfoRow label="Tenant ID" value={route.tenantId} mono />
            <InfoRow label="Created" value={new Date(route.createdAt).toLocaleDateString()} />
          </CardContent>
        </Card>
      </div>

      {/* Edit Dialog */}
      <FormDialog<RouteFormData>
        open={editOpen}
        onOpenChange={setEditOpen}
        title="Edit Route"
        schema={routeSchema}
        defaultValues={{
          name: route.name,
          protocol: 'tcp',
          entry_ip: route.exposedIp,
          gateway_id: route.gatewayId,
          destination_ip: route.privateDestination,
        }}
        fields={routeFields}
        onSubmit={async (data) => {
          await updateRoute.mutateAsync({ id: route.id, body: data });
          setEditOpen(false);
        }}
        isSubmitting={updateRoute.isPending}
        submitLabel="Save Changes"
      />

      {/* Delete Confirmation */}
      <DeleteConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        entityName={route.name}
        onConfirm={async () => {
          await deleteRoute.mutateAsync(route.id);
          navigate('/tenant/routes');
        }}
        isDeleting={deleteRoute.isPending}
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
