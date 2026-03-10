import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { DataTable, Column } from '@/components/ui/DataTable';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { FormDialog, type FieldConfig } from '@/components/FormDialog';
import { DeleteConfirmDialog } from '@/components/DeleteConfirmDialog';
import {
  useCDNSite,
  useCDNOrigins,
  useCreateCDNOrigin,
  useUpdateCDNOrigin,
  useDeleteCDNOrigin,
  useDeleteCDNSite,
  usePurgeCache,
} from '@/hooks/useCDN';
import { cdnOriginSchema, type CDNOriginFormData, cachePurgeSchema, type CachePurgeFormData } from '@/lib/schemas';
import type { CDNOrigin } from '@/types';
import { Layers, ArrowLeft, Trash2, MoreHorizontal, Pencil, RefreshCw } from 'lucide-react';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Label } from '@/components/ui/label';
import { Loader2 } from 'lucide-react';

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
  { name: 'health_check_interval', label: 'Health Check Interval (s)', type: 'number', placeholder: '30' },
];

export default function CDNSiteDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { data: site, isLoading: siteLoading, error } = useCDNSite(id);
  const { data: originsData, isLoading: originsLoading } = useCDNOrigins(id);
  const origins = originsData?.items || [];

  const createOrigin = useCreateCDNOrigin(id || '');
  const updateOrigin = useUpdateCDNOrigin();
  const deleteOrigin = useDeleteCDNOrigin();
  const deleteSite = useDeleteCDNSite();
  const purgeCache = usePurgeCache(id || '');

  const [createOriginOpen, setCreateOriginOpen] = useState(false);
  const [editOriginTarget, setEditOriginTarget] = useState<CDNOrigin | null>(null);
  const [deleteOriginTarget, setDeleteOriginTarget] = useState<CDNOrigin | null>(null);
  const [deleteSiteOpen, setDeleteSiteOpen] = useState(false);

  const purgeForm = useForm<CachePurgeFormData>({
    resolver: zodResolver(cachePurgeSchema),
    defaultValues: { paths: '' },
  });

  if (siteLoading) {
    return (
      <AppLayout>
        <Skeleton className="h-12 w-64 mb-6" />
        <Skeleton className="h-48" />
      </AppLayout>
    );
  }

  if (!site || error) {
    return (
      <AppLayout>
        <div className="flex flex-col items-center justify-center h-[50vh] text-center">
          <Layers className="h-12 w-12 text-muted-foreground mb-4" />
          <h2 className="text-xl font-semibold">CDN Site not found</h2>
          <p className="text-muted-foreground mb-4">The requested CDN site does not exist.</p>
          <Button onClick={() => navigate('/tenant/cdn/services')}>Back to CDN Services</Button>
        </div>
      </AppLayout>
    );
  }

  const originColumns: Column<CDNOrigin>[] = [
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
            <DropdownMenuItem onClick={() => setEditOriginTarget(o)}>
              <Pencil className="mr-2 h-4 w-4" /> Edit Origin
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive" onClick={() => setDeleteOriginTarget(o)}>
              <Trash2 className="mr-2 h-4 w-4" /> Remove Origin
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <AppLayout
      breadcrumbs={[
        { label: 'CDN' },
        { label: 'Services', href: '/tenant/cdn/services' },
        { label: site.name },
      ]}
    >
      <div className="mb-6">
        <Button variant="ghost" size="sm" onClick={() => navigate('/tenant/cdn/services')} className="mb-4 -ml-2">
          <ArrowLeft className="mr-2 h-4 w-4" /> Back to CDN Services
        </Button>

        <div className="flex items-start justify-between">
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
              <Layers className="h-6 w-6 text-primary" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-semibold">{site.name}</h1>
                <StatusBadge status={site.status} />
              </div>
              <p className="text-muted-foreground text-sm">
                {site.domainCount} domains &middot; {site.originCount} origins
              </p>
            </div>
          </div>
          <div className="flex gap-2">
            <Button variant="destructive" size="sm" onClick={() => setDeleteSiteOpen(true)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete Site
            </Button>
          </div>
        </div>
      </div>

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="origins">Origins</TabsTrigger>
          <TabsTrigger value="cache">Cache</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="mt-4">
          <div className="grid gap-4 md:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Site Information</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <InfoRow label="ID" value={site.id} mono />
                <InfoRow label="Name" value={site.name} />
                <InfoRow label="Domains" value={String(site.domainCount)} />
                <InfoRow label="Origins" value={String(site.originCount)} />
                <InfoRow label="Created" value={new Date(site.createdAt).toLocaleDateString()} />
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">Traffic</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <InfoRow label="Bandwidth" value={`${site.bandwidthGb.toFixed(1)} GB`} />
                <InfoRow label="Requests" value={`${site.requestsM.toFixed(1)}M`} />
                <InfoRow label="Status" value={site.status} />
                <InfoRow label="Tenant ID" value={site.tenantId} mono />
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="origins" className="mt-4">
          <PageHeader
            title="Origins"
            description={`${originsData?.total ?? 0} origin servers`}
            action={{ label: 'Add Origin', onClick: () => setCreateOriginOpen(true) }}
          />

          {originsLoading ? (
            <Skeleton className="h-64" />
          ) : (
            <DataTable
              data={origins}
              columns={originColumns}
              searchKeys={['name', 'hostname']}
              pageSize={10}
              emptyMessage="No origins configured for this site"
            />
          )}
        </TabsContent>

        <TabsContent value="cache" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Purge Cache</CardTitle>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={purgeForm.handleSubmit(async (data) => {
                  await purgeCache.mutateAsync(data);
                  purgeForm.reset();
                })}
                className="space-y-4"
              >
                <div className="space-y-2">
                  <Label htmlFor="paths">Paths to purge</Label>
                  <textarea
                    id="paths"
                    className="flex min-h-[120px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 font-mono"
                    placeholder={'/images/*\n/css/style.css\n/api/v1/*'}
                    {...purgeForm.register('paths')}
                  />
                  {purgeForm.formState.errors.paths && (
                    <p className="text-xs text-destructive">{purgeForm.formState.errors.paths.message}</p>
                  )}
                  <p className="text-xs text-muted-foreground">Enter one path per line. Wildcards (*) are supported.</p>
                </div>
                <Button type="submit" disabled={purgeCache.isPending}>
                  {purgeCache.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                  <RefreshCw className="mr-2 h-4 w-4" /> Purge Cache
                </Button>
              </form>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Create Origin Dialog */}
      <FormDialog<CDNOriginFormData>
        open={createOriginOpen}
        onOpenChange={setCreateOriginOpen}
        title="Add Origin"
        description="Add a new origin server for this CDN site."
        schema={cdnOriginSchema}
        defaultValues={{ address: '', scheme: 'https', weight: 100, health_check_path: '', health_check_interval: 30 }}
        fields={originFields}
        onSubmit={async (data) => {
          await createOrigin.mutateAsync(data);
          setCreateOriginOpen(false);
        }}
        isSubmitting={createOrigin.isPending}
        submitLabel="Add Origin"
      />

      {/* Edit Origin Dialog */}
      {editOriginTarget && (
        <FormDialog<CDNOriginFormData>
          open={!!editOriginTarget}
          onOpenChange={(open) => !open && setEditOriginTarget(null)}
          title="Edit Origin"
          schema={cdnOriginSchema}
          defaultValues={{
            address: editOriginTarget.hostname,
            scheme: editOriginTarget.protocol,
            weight: editOriginTarget.weight,
          }}
          fields={originFields}
          onSubmit={async (data) => {
            await updateOrigin.mutateAsync({ id: editOriginTarget.id, body: data });
            setEditOriginTarget(null);
          }}
          isSubmitting={updateOrigin.isPending}
          submitLabel="Save Changes"
        />
      )}

      {/* Delete Origin Confirmation */}
      <DeleteConfirmDialog
        open={!!deleteOriginTarget}
        onOpenChange={(open) => !open && setDeleteOriginTarget(null)}
        entityName={deleteOriginTarget?.name}
        onConfirm={async () => {
          if (deleteOriginTarget) {
            await deleteOrigin.mutateAsync(deleteOriginTarget.id);
            setDeleteOriginTarget(null);
          }
        }}
        isDeleting={deleteOrigin.isPending}
      />

      {/* Delete Site Confirmation */}
      <DeleteConfirmDialog
        open={deleteSiteOpen}
        onOpenChange={setDeleteSiteOpen}
        title="Delete CDN Site"
        description={`Are you sure you want to delete "${site.name}" and all its origins? This action cannot be undone.`}
        onConfirm={async () => {
          await deleteSite.mutateAsync(site.id);
          navigate('/tenant/cdn/services');
        }}
        isDeleting={deleteSite.isPending}
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
