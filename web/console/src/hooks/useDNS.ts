import { useQuery } from '@tanstack/react-query';
import { apiList } from '@/lib/api';
import { transformDNSZone, transformDNSRecord } from '@/lib/transforms';
import type { ApiDNSZone, ApiDNSRecord } from '@/types/api';
import type { DNSZone, DNSRecord } from '@/types';
import type { ListResult } from '@/lib/api';

export function useDNSZones(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['dnsZones', params],
    queryFn: async (): Promise<ListResult<DNSZone>> => {
      const result = await apiList<ApiDNSZone>('/api/v1/dns/zones', params);
      return { ...result, items: result.items.map(transformDNSZone) };
    },
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
