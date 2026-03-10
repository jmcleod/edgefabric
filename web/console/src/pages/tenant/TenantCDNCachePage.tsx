import { useState } from 'react';
import { AppLayout } from '@/components/layout/AppLayout';
import { PageHeader } from '@/components/ui/PageHeader';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { useAuth } from '@/hooks/useAuth';
import { useCDNSites, usePurgeCache } from '@/hooks/useCDN';
import { cachePurgeSchema, type CachePurgeFormData } from '@/lib/schemas';
import { Disc, RefreshCw, Loader2 } from 'lucide-react';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';

export default function TenantCDNCachePage() {
  const { user } = useAuth();
  const tenantId = user?.tenantId || '';
  const { data: sitesData, isLoading: sitesLoading } = useCDNSites(tenantId || undefined);
  const sites = sitesData?.items || [];

  const [selectedSiteId, setSelectedSiteId] = useState<string>('');
  const purgeCache = usePurgeCache(selectedSiteId);

  const purgeForm = useForm<CachePurgeFormData>({
    resolver: zodResolver(cachePurgeSchema),
    defaultValues: { paths: '' },
  });

  return (
    <AppLayout breadcrumbs={[{ label: 'CDN' }, { label: 'Cache & Purge' }]}>
      <PageHeader
        title="Cache & Purge"
        description="Manage CDN cache for your services"
        icon={Disc}
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
          Select a CDN service above to purge its cache.
        </div>
      ) : (
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
      )}
    </AppLayout>
  );
}
