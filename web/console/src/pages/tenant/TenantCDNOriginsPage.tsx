import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useAuth } from '@/hooks/useAuth';
import { useCDNSites, useCDNOrigins, useCreateCDNOrigin, useDeleteCDNOrigin } from '@/hooks/useCDN';
import { cdnOriginSchema, type CDNOriginFormData } from '@/lib/schemas';
import type { CDNOrigin } from '@/types';
import { HardDrive, MoreHorizontal, Trash2 } from 'lucide-react';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

const originFields: FieldConfig<CDNOriginFormData>[] = [
  { name: 'address', label: 'Origin Address', placeholder: 'origin.example.com' },
  {
    name: 'scheme',
    label: 'Scheme',
    type: 'select',
    options: [
      { label: 'HTTPS', value: 'https' },
      { label: 'HTTP', value: 'http' },
    ],
  },
  { name: 'weight', label: 'Weight', type: 'number', placeholder: '100' },
  { name: 'health_check_path', label: 'Health Check Path', placeholder: '/health (optional)' },
];

export default function TenantCDNOriginsPage() {
  const { user } = useAuth();
  const tenantId = user?.tenantId || '';
  const { data: sitesData } = useCDNSites(tenantId || undefined);
  const sites = sitesData?.items || [];

  const [selectedSiteId, setSelectedSiteId] = useState<string>('');
  const { data: originsData, isLoading } = useCDNOrigins(selectedSiteId || undefined);
  const origins = originsData?.items || [];

  const createOrigin = useCreateCDNOrigin(selectedSiteId);
  const deleteOrigin = useDeleteCDNOrigin();

  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<CDNOrigin | null>(null);

  const columns: Column<CDNOrigin>[] = [
    {
      key: 'name',
      header: 'Origin',
      render: (o) => (
        <div>
          <p className="font-medium text-foreground">{o.name}</p>
          <p className="text-xs text-muted-foreground mono-data">{o.hostname}:{o.port}</p>
        </div>
      ),
    },
    {
      key: 'protocol',
      header: 'Protocol',
      render: (o) => (
        <span className="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-xs font-medium uppercase">
          {o.protocol}
        </span>
      ),
    },
    {
      key: 'weight',
      header: 'Weight',
      render: (o) => <span className="text-sm">{o.weight}</span>,
    },
    {
      key: 'status',
      header: 'Health',
      render: (o) => <StatusBadge status={o.status} size="sm" />,
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (o) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(o)}>
              <Trash2 className="mr-2 h-4 w-4" /> Remove Origin
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'CDN' }, { label: 'Origins' }]}>
      <PageHeader
        title="CDN Origins"
        description="Origin servers for your CDN services"
        icon={HardDrive}
        action={selectedSiteId ? { label: 'Add Origin', onClick: () => setCreateOpen(true) } : undefined}
      />

      <div className="mb-4 max-w-xs">
        <Select value={selectedSiteId} onValueChange={setSelectedSiteId}>
          <SelectTrigger>
            <SelectValue placeholder="Select a CDN service..." />
          </SelectTrigger>
          <SelectContent>
            {sites.map((site) => (
              <SelectItem key={site.id} value={site.id}>
                {site.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {!selectedSiteId ? (
        <div className="text-center py-16 text-muted-foreground">
          Select a CDN service above to manage its origins.
        </div>
      ) : isLoading ? (
        <Skeleton className="h-64" />
      ) : (
        <DataTable
          data={origins}
          columns={columns}
          searchKeys={['name', 'hostname']}
          pageSize={10}
          emptyMessage="No origins configured for this service"
        />
      )}

      <FormDialog<CDNOriginFormData>
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Add Origin"
        schema={cdnOriginSchema}
        defaultValues={{ address: '', scheme: 'https', weight: 100, health_check_path: '' }}
        fields={originFields}
        onSubmit={async (data) => {
          await createOrigin.mutateAsync(data);
          setCreateOpen(false);
        }}
        isSubmitting={createOrigin.isPending}
        submitLabel="Add Origin"
      />

      <DeleteConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        entityName={deleteTarget?.name}
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteOrigin.mutateAsync(deleteTarget.id);
            setDeleteTarget(null);
          }
        }}
        isDeleting={deleteOrigin.isPending}
      />
    </AppLayout>
  );
}
