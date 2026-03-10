import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useGateways, useCreateGateway, useDeleteGateway } from '@/hooks/useGateways';
import { gatewaySchema, type GatewayFormData } from '@/lib/schemas';
import type { Gateway } from '@/types';
import { Waypoints, Eye, MoreHorizontal, Trash2 } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { formatDistanceToNow } from 'date-fns';

const gatewayFields: FieldConfig<GatewayFormData>[] = [
  { name: 'name', label: 'Name', placeholder: 'us-east-gw-01' },
  { name: 'tenant_id', label: 'Tenant ID', placeholder: 'Tenant UUID' },
  { name: 'public_ip', label: 'Public IP', placeholder: '203.0.113.1 (optional)' },
];

export default function GatewaysPage() {
  const navigate = useNavigate();
  const { data, isLoading } = useGateways();
  const gateways = data?.items || [];
  const createGateway = useCreateGateway();
  const deleteGateway = useDeleteGateway();

  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Gateway | null>(null);

  const columns: Column<Gateway>[] = [
    {
      key: 'name',
      header: 'Gateway',
      render: (gw) => (
        <div>
          <p className="font-medium text-foreground">{gw.name}</p>
          <p className="text-xs text-muted-foreground mono-data">{gw.publicIp}</p>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (gw) => <StatusBadge status={gw.status} size="sm" />,
    },
    {
      key: 'lastSeen',
      header: 'Last Seen',
      render: (gw) => (
        <span className="text-muted-foreground text-sm">
          {gw.lastSeen
            ? formatDistanceToNow(new Date(gw.lastSeen), { addSuffix: true })
            : '\u2014'}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (gw) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => navigate(`/gateways/${gw.id}`)}>
              <Eye className="mr-2 h-4 w-4" /> View Details
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(gw)}>
              <Trash2 className="mr-2 h-4 w-4" /> Remove Gateway
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Infrastructure' }, { label: 'Gateways' }]}>
      <PageHeader
        title="Gateways"
        description="WireGuard gateway nodes for private routing"
        icon={Waypoints}
        action={{
          label: 'Add Gateway',
          onClick: () => setCreateOpen(true),
        }}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={gateways}
          columns={columns}
          searchKeys={['name', 'publicIp']}
          pageSize={10}
          onRowClick={(gw) => navigate(`/gateways/${gw.id}`)}
        />
      )}

      {/* Create Dialog */}
      <FormDialog<GatewayFormData>
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Add Gateway"
        description="Register a new WireGuard gateway."
        schema={gatewaySchema}
        defaultValues={{ name: '', tenant_id: '', public_ip: '' }}
        fields={gatewayFields}
        onSubmit={async (data) => {
          await createGateway.mutateAsync(data);
          setCreateOpen(false);
        }}
        isSubmitting={createGateway.isPending}
        submitLabel="Add Gateway"
      />

      {/* Delete Confirmation */}
      <DeleteConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        entityName={deleteTarget?.name}
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteGateway.mutateAsync(deleteTarget.id);
            setDeleteTarget(null);
          }
        }}
        isDeleting={deleteGateway.isPending}
      />
    </AppLayout>
  );
}
