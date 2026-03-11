import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import { useCDNSites, useCreateCDNSite, useDeleteCDNSite } from '@/hooks/useCDN';
import { useAuth } from '@/hooks/useAuth';
import { cdnSiteSchema, type CDNSiteFormData } from '@/lib/schemas';
import type { CDNService } from '@/types';
import { Layers, Eye, MoreHorizontal, Globe, Trash2 } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

export const siteFields: FieldConfig<CDNSiteFormData>[] = [
  { name: 'name', label: 'Name', placeholder: 'my-cdn-site' },
  { name: 'domains', label: 'Domains', placeholder: 'cdn.example.com (comma-separated)', description: 'Comma-separated list of domains' },
  {
    name: 'tls_mode',
    label: 'TLS Mode',
    type: 'select',
    options: [
      { label: 'Auto (Let\'s Encrypt)', value: 'auto' },
      { label: 'Manual', value: 'manual' },
      { label: 'Disabled', value: 'disabled' },
    ],
  },
  { name: 'cache_enabled', label: 'Enable Caching', type: 'switch' },
  { name: 'cache_ttl', label: 'Cache TTL (seconds)', type: 'number', placeholder: '3600' },
  { name: 'compression_enabled', label: 'Enable Compression (Brotli + Gzip)', type: 'switch' },
  { name: 'rate_limit_rps', label: 'Rate Limit (req/s)', type: 'number', placeholder: '0 = unlimited' },
  { name: 'waf_enabled', label: 'Enable WAF', type: 'switch', description: 'Web Application Firewall protects against SQLi, XSS, and path traversal attacks' },
  {
    name: 'waf_mode',
    label: 'WAF Mode',
    type: 'select',
    options: [
      { label: 'Detect Only (log matches)', value: 'detect' },
      { label: 'Block (return 403)', value: 'block' },
    ],
    visibleWhen: { field: 'waf_enabled', value: true },
  },
  { name: 'node_group_id', label: 'Node Group ID', placeholder: 'Optional — assign to a node group' },
];

function FeatureBadge({ label, enabled }: { label: string; enabled: boolean }) {
  return (
    <span
      className={`inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium ${
        enabled
          ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
          : 'bg-muted text-muted-foreground'
      }`}
    >
      {label}
    </span>
  );
}

export default function CDNServicesPage() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const tenantId = user?.tenantId || '';
  const { data, isLoading } = useCDNSites(tenantId || undefined);
  const cdnServices = data?.items || [];
  const createSite = useCreateCDNSite(tenantId);
  const deleteSite = useDeleteCDNSite();

  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<CDNService | null>(null);

  const columns: Column<CDNService>[] = [
    {
      key: 'name',
      header: 'Service',
      render: (service) => (
        <div>
          <p className="font-medium text-foreground">{service.name}</p>
          <p className="text-xs text-muted-foreground mono-data">{service.id}</p>
        </div>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      render: (service) => <StatusBadge status={service.status} size="sm" />,
    },
    {
      key: 'domainCount',
      header: 'Domains',
      render: (service) => (
        <div className="flex items-center gap-1.5">
          <Globe className="h-3.5 w-3.5 text-muted-foreground" />
          <span>{service.domainCount}</span>
        </div>
      ),
    },
    {
      key: 'features',
      header: 'Features',
      render: (service) => (
        <div className="flex flex-wrap gap-1">
          <FeatureBadge label="Cache" enabled={service.cacheEnabled} />
          <FeatureBadge label="Brotli" enabled={service.compressionEnabled} />
          <FeatureBadge label="WAF" enabled={service.wafEnabled} />
        </div>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-12',
      render: (service) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => navigate(`/tenant/cdn/services/${service.id}`)}>
              <Eye className="mr-2 h-4 w-4" /> View Details
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(service)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete Service
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout breadcrumbs={[{ label: 'CDN' }, { label: 'Services' }]}>
      <PageHeader
        title="CDN Services"
        description={`${data?.total ?? 0} services`}
        icon={Layers}
        action={{
          label: 'Create Service',
          onClick: () => setCreateOpen(true),
        }}
      />

      {isLoading ? (
        <Skeleton className="h-96" />
      ) : (
        <DataTable
          data={cdnServices}
          columns={columns}
          searchKeys={['name']}
          pageSize={10}
          onRowClick={(service) => navigate(`/tenant/cdn/services/${service.id}`)}
        />
      )}

      <FormDialog<CDNSiteFormData>
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Create CDN Service"
        description="Set up a new CDN site for content delivery."
        schema={cdnSiteSchema}
        defaultValues={{ name: '', domains: '', tls_mode: 'auto', cache_enabled: true, cache_ttl: 3600, compression_enabled: true, rate_limit_rps: 0, waf_enabled: false, waf_mode: 'detect', node_group_id: '' }}
        fields={siteFields}
        onSubmit={async (data) => {
          await createSite.mutateAsync(data);
          setCreateOpen(false);
        }}
        isSubmitting={createSite.isPending}
        submitLabel="Create Service"
      />

      <DeleteConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        entityName={deleteTarget?.name}
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteSite.mutateAsync(deleteTarget.id);
            setDeleteTarget(null);
          }
        }}
        isDeleting={deleteSite.isPending}
      />
    </AppLayout>
  );
}
