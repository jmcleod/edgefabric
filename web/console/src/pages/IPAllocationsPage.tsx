import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useTenants } from '@/hooks/useTenants';
import { useIPAllocations, useCreateIPAllocation, useDeleteIPAllocation } from '@/hooks/useIPAllocations';
import { ipAllocationSchema, type IPAllocationFormData } from '@/lib/schemas';
import type { IPAllocation } from '@/types';
import { Globe, MoreHorizontal, Trash2 } from 'lucide-react';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

const allocationFields: FieldConfig<IPAllocationFormData>[] = [
  { name: 'prefix', label: 'IP Prefix', placeholder: '192.168.1.0/24' },
  { name: 'type', label: 'Type', type: 'select', options: [
    { label: 'Anycast', value: 'anycast' },
    { label: 'Unicast', value: 'unicast' },
  ] },
  { name: 'purpose', label: 'Purpose', type: 'select', options: [
    { label: 'DNS', value: 'dns' },
    { label: 'CDN', value: 'cdn' },
    { label: 'Route', value: 'route' },
  ] },
];

export default function IPAllocationsPage() {
  const { data: tenantsData } = useTenants();
  const tenants = tenantsData?.items || [];
  const [selectedTenantId, setSelectedTenantId] = useState<string>('');

  const { data, isLoading } = useIPAllocations(selectedTenantId || undefined);
  const allocations = data?.items || [];
  const createAllocation = useCreateIPAllocation(selectedTenantId || undefined);
  const deleteAllocation = useDeleteIPAllocation();

  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<IPAllocation | null>(null);

  const columns: Column<IPAllocation>[] = [
    {
      key: 'prefix',
      header: 'IP Prefix',
      render: (alloc) => <code className="mono-data text-sm">{alloc.prefix}</code>,
    },
    {
      key: 'type',
      header: 'Type',
      render: (alloc) => <Badge variant="secondary">{alloc.type}</Badge>,
    },
    {
      key: 'purpose',
      header: 'Purpose',
      render: (alloc) => <Badge variant="outline">{alloc.purpose}</Badge>,
    },
    {
      key: 'status',
      header: 'Status',
      render: (alloc) => (
        <StatusBadge
          status={alloc.status === 'active' ? 'healthy' : alloc.status === 'pending' ? 'syncing' : 'critical'}
          size="sm"
        />
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (alloc) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(alloc)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'Networking' }, { label: 'Advertised IPs' }]}>
      <PageHeader
        title="Advertised IPs"
        description="IP prefix allocations for BGP announcement"
        icon={Globe}
        action={selectedTenantId ? { label: 'Add IP', onClick: () => setCreateOpen(true) } : undefined}
      />

      <div className="mb-4 max-w-xs">
        <Select value={selectedTenantId} onValueChange={setSelectedTenantId}>
          <SelectTrigger>
            <SelectValue placeholder="Select a tenant..." />
          </SelectTrigger>
          <SelectContent>
            {tenants.map((t) => (
              <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {!selectedTenantId ? (
        <div className="text-center py-16 text-muted-foreground">
          Select a tenant above to view IP allocations.
        </div>
      ) : isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable data={allocations} columns={columns} searchKeys={['prefix']} pageSize={10} emptyMessage="No IP allocations for this tenant" />
      )}

      <FormDialog<IPAllocationFormData>
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Add IP Allocation"
        schema={ipAllocationSchema}
        defaultValues={{ prefix: '', type: 'anycast', purpose: 'dns' }}
        fields={allocationFields}
        onSubmit={async (data) => { await createAllocation.mutateAsync(data); setCreateOpen(false); }}
        isSubmitting={createAllocation.isPending}
        submitLabel="Add IP"
      />

      <DeleteConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        entityName={deleteTarget?.prefix}
        onConfirm={async () => { if (deleteTarget) { await deleteAllocation.mutateAsync(deleteTarget.id); setDeleteTarget(null); } }}
        isDeleting={deleteAllocation.isPending}
      />
    </AppLayout>
  );
}
