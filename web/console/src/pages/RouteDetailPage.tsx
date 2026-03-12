import { useState, useMemo } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { AppLayout } from '@/components/layout/AppLayout';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { CopyableText } from '@/components/ui/CopyableText';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useRoute, useUpdateRoute, useDeleteRoute } from '@/hooks/useRoutes';
import { useGateways } from '@/hooks/useGateways';
import { useNodeGroups } from '@/hooks/useNodeGroups';
import { useTenants } from '@/hooks/useTenants';
import { routeSchema, type RouteFormData } from '@/lib/schemas';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { ArrowRightLeft, ArrowLeft, Pencil, Trash2, Info } from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';

export default function RouteDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { data: route, isLoading, error } = useRoute(id);
  const updateRoute = useUpdateRoute();
  const deleteRoute = useDeleteRoute();

  const { data: gatewaysData } = useGateways();
  const { data: nodeGroupsData } = useNodeGroups();
  const { data: tenantsData } = useTenants();

  const gatewayOptions = useMemo(
    () => (gatewaysData?.items || []).map((gw) => ({ label: gw.name, value: gw.id, description: gw.id.slice(0, 8) + '...' })),
    [gatewaysData],
  );
  const nodeGroupOptions = useMemo(
    () => (nodeGroupsData?.items || []).map((ng) => ({ label: ng.name, value: ng.id, description: ng.id.slice(0, 8) + '...' })),
    [nodeGroupsData],
  );
  const gatewayMap = useMemo(
    () => Object.fromEntries((gatewaysData?.items || []).map((gw) => [gw.id, gw.name])),
    [gatewaysData],
  );
  const tenantMap = useMemo(
    () => Object.fromEntries((tenantsData?.items || []).map((t) => [t.id, t.name])),
    [tenantsData],
  );

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
    { name: 'gateway_id', label: 'Gateway', type: 'combobox', comboboxOptions: gatewayOptions, placeholder: 'Select a gateway...' },
    { name: 'destination_ip', label: 'Destination IP', placeholder: '10.0.1.5' },
    { name: 'destination_port', label: 'Destination Port', type: 'number', placeholder: '8443' },
    { name: 'node_group_id', label: 'Node Group', type: 'combobox', comboboxOptions: nodeGroupOptions, placeholder: 'Select a node group...', clearable: true },
  ];

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

      {route.protocol === 'icmp' && (
        <Alert className="mb-4">
          <Info className="h-4 w-4" />
          <AlertDescription>
            ICMP forwarding requires <code className="mono-data text-xs">CAP_NET_RAW</code> on both nodes and gateways. Port fields are not applicable for ICMP routes.
          </AlertDescription>
        </Alert>
      )}

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Route Configuration</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <InfoRow label="ID"><CopyableText value={route.id} /></InfoRow>
            <InfoRow label="Name">{route.name}</InfoRow>
            <InfoRow label="Protocol">{route.protocol.toUpperCase()}</InfoRow>
            <InfoRow label="Entry IP"><CopyableText value={route.exposedIp} /></InfoRow>
            <InfoRow label="Destination"><CopyableText value={route.privateDestination} /></InfoRow>
            <InfoRow label="Gateway">
              <Link to={`/gateways/${route.gatewayId}`} className="text-primary hover:underline text-sm">
                {gatewayMap[route.gatewayId] || route.gatewayId}
              </Link>
            </InfoRow>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Metadata</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <InfoRow label="Status"><StatusBadge status={route.status} size="sm" /></InfoRow>
            <InfoRow label="Tenant">
              <Link to={`/tenants/${route.tenantId}`} className="text-primary hover:underline text-sm">
                {tenantMap[route.tenantId] || route.tenantId}
              </Link>
            </InfoRow>
            <InfoRow label="Created">
              {formatDistanceToNow(new Date(route.createdAt), { addSuffix: true })}
            </InfoRow>
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

function InfoRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex justify-between items-center py-1 border-b border-border/50 last:border-0">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="text-sm">{children}</span>
    </div>
  );
}
