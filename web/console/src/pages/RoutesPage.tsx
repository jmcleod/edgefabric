import { useState, useMemo } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { EmptyState } from '@/components/ui/EmptyState';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useRoutes, useCreateRoute, useDeleteRoute } from '@/hooks/useRoutes';
import { useGateways } from '@/hooks/useGateways';
import { useNodeGroups } from '@/hooks/useNodeGroups';
import { useAuth } from '@/hooks/useAuth';
import { routeSchema, type RouteFormData } from '@/lib/schemas';
import type { Route } from '@/types';
import { ArrowRightLeft, MoreHorizontal, Trash2, Eye } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

export default function RoutesPage() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const tenantId = user?.tenantId || '';
  const { data, isLoading } = useRoutes(tenantId || undefined);
  const routes = data?.items || [];
  const createRoute = useCreateRoute(tenantId);
  const deleteRoute = useDeleteRoute();

  const { data: gatewaysData } = useGateways();
  const { data: nodeGroupsData } = useNodeGroups();

  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Route | null>(null);

  const gatewayOptions = useMemo(
    () =>
      (gatewaysData?.items || []).map((gw) => ({
        value: gw.id,
        label: gw.name,
        description: gw.id.slice(0, 8) + '...',
      })),
    [gatewaysData],
  );

  const nodeGroupOptions = useMemo(
    () =>
      (nodeGroupsData?.items || []).map((ng) => ({
        value: ng.id,
        label: ng.name,
        description: ng.id.slice(0, 8) + '...',
      })),
    [nodeGroupsData],
  );

  const gatewayMap = useMemo(
    () =>
      Object.fromEntries(
        (gatewaysData?.items || []).map((gw) => [gw.id, gw.name]),
      ),
    [gatewaysData],
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
    {
      name: 'gateway_id',
      label: 'Gateway',
      type: 'combobox',
      comboboxOptions: gatewayOptions,
      placeholder: 'Select a gateway...',
    },
    { name: 'destination_ip', label: 'Destination IP', placeholder: '10.0.1.5' },
    { name: 'destination_port', label: 'Destination Port', type: 'number', placeholder: '8443' },
    {
      name: 'node_group_id',
      label: 'Node Group',
      type: 'combobox',
      comboboxOptions: nodeGroupOptions,
      placeholder: 'Select a node group...',
      clearable: true,
    },
  ];

  const columns: Column<Route>[] = [
    {
      key: 'name',
      header: 'Route',
      render: (r) => (
        <div>
          <p className="font-medium text-foreground">{r.name}</p>
          <p className="text-xs text-muted-foreground mono-data">{r.id}</p>
        </div>
      ),
    },
    {
      key: 'exposedIp',
      header: 'Entry',
      render: (r) => <code className="mono-data text-sm">{r.exposedIp}</code>,
    },
    {
      key: 'privateDestination',
      header: 'Destination',
      render: (r) => <code className="mono-data text-sm">{r.privateDestination}</code>,
    },
    {
      key: 'gatewayId',
      header: 'Gateway',
      render: (r) => (
        <span className="text-sm text-foreground">
          {gatewayMap[r.gatewayId] || r.gatewayId?.slice(0, 8) + '...'}
        </span>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (r) => <StatusBadge status={r.status} size="sm" />,
    },
    {
      key: 'createdAt',
      header: 'Created',
      render: (r) => (
        <span className="text-sm text-muted-foreground">
          {formatDistanceToNow(new Date(r.createdAt), { addSuffix: true })}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (r) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => navigate(`/tenant/routes/${r.id}`)}>
              <Eye className="mr-2 h-4 w-4" /> View Details
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(r)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete Route
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Networking' }, { label: 'Routes' }]}>
      <PageHeader
        title="Routes"
        description={`${data?.total ?? 0} private routes`}
        icon={ArrowRightLeft}
        action={{
          label: 'Create Route',
          onClick: () => setCreateOpen(true),
        }}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : routes.length === 0 ? (
        <EmptyState
          icon={ArrowRightLeft}
          title="No routes configured"
          description="Create a route to define private traffic paths through your gateways."
          action={{
            label: 'Create Route',
            onClick: () => setCreateOpen(true),
          }}
        />
      ) : (
        <DataTable
          data={routes}
          columns={columns}
          searchKeys={['name', 'exposedIp', 'privateDestination']}
          pageSize={10}
          onRowClick={(r) => navigate(`/tenant/routes/${r.id}`)}
          emptyMessage="No routes configured"
        />
      )}

      <FormDialog<RouteFormData>
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Create Route"
        description="Define a new private route through a gateway."
        schema={routeSchema}
        defaultValues={{ name: '', protocol: 'tcp', entry_ip: '', gateway_id: '', destination_ip: '', node_group_id: '' }}
        fields={routeFields}
        onSubmit={async (data) => {
          await createRoute.mutateAsync(data);
          setCreateOpen(false);
        }}
        isSubmitting={createRoute.isPending}
        submitLabel="Create Route"
      />

      <DeleteConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        entityName={deleteTarget?.name}
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteRoute.mutateAsync(deleteTarget.id);
            setDeleteTarget(null);
          }
        }}
        isDeleting={deleteRoute.isPending}
      />
    </AppLayout>
  );
}
