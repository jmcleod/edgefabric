import { useQuery } from '@tanstack/react-query';
import { apiList } from '@/lib/api';
import { transformCDNSite, transformCDNOrigin } from '@/lib/transforms';
import type { ApiCDNSite, ApiCDNOrigin } from '@/types/api';
import type { CDNService, CDNOrigin } from '@/types';
import type { ListResult } from '@/lib/api';

export function useCDNSites(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['cdnSites', params],
    queryFn: async (): Promise<ListResult<CDNService>> => {
      const result = await apiList<ApiCDNSite>('/api/v1/cdn/sites', params);
      return { ...result, items: result.items.map(transformCDNSite) };
    },
  });
}

export function useCDNOrigins(siteId: string | undefined) {
  return useQuery({
    queryKey: ['cdnOrigins', siteId],
    queryFn: async (): Promise<ListResult<CDNOrigin>> => {
      const result = await apiList<ApiCDNOrigin>(`/api/v1/cdn/sites/${siteId}/origins`, { limit: 100 });
      return { ...result, items: result.items.map(transformCDNOrigin) };
    },
    enabled: !!siteId,
  });
}
