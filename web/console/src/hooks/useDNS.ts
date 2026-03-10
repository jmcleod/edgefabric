import { useQuery } from '@tanstack/react-query';
import { apiList, apiGet } from '@/lib/api';
import { transformDNSZone, transformDNSRecord } from '@/lib/transforms';
import type { ApiDNSZone, ApiDNSRecord } from '@/types/api';
import type { DNSZone, DNSRecord } from '@/types';
import type { ListResult } from '@/lib/api';

export function useDNSZones(tenantId: string | undefined, params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['dnsZones', tenantId, params],
    queryFn: async (): Promise<ListResult<DNSZone>> => {
      const result = await apiList<ApiDNSZone>(`/api/v1/tenants/${tenantId}/dns/zones`, params);
      return { ...result, items: result.items.map(transformDNSZone) };
    },
    enabled: !!tenantId,
  });
}

export function useDNSZone(zoneId: string | undefined) {
  return useQuery({
    queryKey: ['dnsZone', zoneId],
    queryFn: async () => {
      const api = await apiGet<ApiDNSZone>(`/api/v1/dns/zones/${zoneId}`);
      return transformDNSZone(api);
    },
    enabled: !!zoneId,
  });
}

export function useDNSRecords(zoneId: string | undefined) {
  return useQuery({
    queryKey: ['dnsRecords', zoneId],
    queryFn: async (): Promise<ListResult<DNSRecord>> => {
      const result = await apiList<ApiDNSRecord>(`/api/v1/dns/zones/${zoneId}/records`, { limit: 200 });
      return { ...result, items: result.items.map(transformDNSRecord) };
    },
    enabled: !!zoneId,
  });
}
