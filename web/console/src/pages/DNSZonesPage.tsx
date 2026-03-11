import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useDNSZones, useCreateDNSZone, useDeleteDNSZone } from '@/hooks/useDNS';
import { useAuth } from '@/hooks/useAuth';
import { dnsZoneSchema, type DNSZoneFormData } from '@/lib/schemas';
import type { DNSZone } from '@/types';
import { Globe, Eye, MoreHorizontal, Trash2 } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { formatDistanceToNow } from 'date-fns';

const zoneFields: FieldConfig<DNSZoneFormData>[] = [
  { name: 'name', label: 'Zone Name', placeholder: 'example.com' },
  { name: 'ttl', label: 'Default TTL (seconds)', type: 'number', placeholder: '3600' },
  { name: 'node_group_id', label: 'Node Group ID', placeholder: 'Optional — assign to a node group' },
  { name: 'transfer_allowed_ips', label: 'AXFR Allowed IPs', placeholder: '10.0.0.1, 192.168.1.0/24', description: 'Comma-separated IPs or CIDRs allowed to perform zone transfers. Leave empty to disable AXFR.' },
];

export default function DNSZonesPage() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const tenantId = user?.tenantId || '';
  const { data, isLoading } = useDNSZones(tenantId || undefined);
  const dnsZones = data?.items || [];
  const createZone = useCreateDNSZone(tenantId);
  const deleteZone = useDeleteDNSZone();

  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<DNSZone | null>(null);

  const columns: Column<DNSZone>[] = [
    {
      key: 'name',
      header: 'Zone',
      render: (zone) => (
        <div>
          <p className="font-medium text-foreground">{zone.name}</p>
          <p className="text-xs text-muted-foreground mono-data">Serial: {zone.serial}</p>
        </div>
      ),
    },
    {
      key: 'recordCount',
      header: 'Records',
      render: (zone) => <span className="text-sm">{zone.recordCount}</span>,
    },
    {
      key: 'status',
      header: 'Status',
      render: (zone) => <StatusBadge status={zone.status} size="sm" />,
    },
    {
      key: 'lastModified',
      header: 'Last Modified',
      render: (zone) => (
        <span className="text-muted-foreground text-sm">
          {formatDistanceToNow(new Date(zone.lastModified), { addSuffix: true })}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (zone) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => navigate(`/tenant/dns/zones/${zone.id}`)}>
              <Eye className="mr-2 h-4 w-4" /> View Records
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(zone)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete Zone
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'DNS' }, { label: 'Zones' }]}>
      <PageHeader
        title="DNS Zones"
        description={`${data?.total ?? 0} zones`}
        icon={Globe}
        action={{
          label: 'Add Zone',
          onClick: () => setCreateOpen(true),
        }}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={dnsZones}
          columns={columns}
          searchKeys={['name']}
          pageSize={10}
          onRowClick={(zone) => navigate(`/tenant/dns/zones/${zone.id}`)}
        />
      )}

      <FormDialog<DNSZoneFormData>
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Add DNS Zone"
        description="Create a new authoritative DNS zone."
        schema={dnsZoneSchema}
        defaultValues={{ name: '', ttl: 3600, node_group_id: '', transfer_allowed_ips: '' }}
        fields={zoneFields}
        onSubmit={async (data) => {
          await createZone.mutateAsync(data);
          setCreateOpen(false);
        }}
        isSubmitting={createZone.isPending}
        submitLabel="Create Zone"
      />

      <DeleteConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        entityName={deleteTarget?.name}
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteZone.mutateAsync(deleteTarget.id);
            setDeleteTarget(null);
          }
        }}
        isDeleting={deleteZone.isPending}
      />
    </AppLayout>
  );
}
