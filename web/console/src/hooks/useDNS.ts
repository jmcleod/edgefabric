import { useQuery } from '@tanstack/react-query';
import { apiList, apiGet } from '@/lib/api';
import { transformDNSZone, transformDNSRecord } from '@/lib/transforms';
import { useCreateMutation, useUpdateMutation, useDeleteMutation } from './useMutations';
import type { ApiDNSZone, ApiDNSRecord } from '@/types/api';
import type { DNSZone, DNSRecord } from '@/types';
import type { ListResult } from '@/lib/api';
import type { DNSZoneFormData, DNSRecordFormData } from '@/lib/schemas';

// --- Zone queries ---

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

// --- Zone mutations ---

export function useCreateDNSZone(tenantId: string) {
  return useCreateMutation<DNSZoneFormData>(`/api/v1/tenants/${tenantId}/dns/zones`, {
    invalidateKeys: [['dnsZones']],
    successMessage: 'DNS zone created',
  });
}

export function useUpdateDNSZone() {
  return useUpdateMutation<Partial<DNSZoneFormData>>(
    (id) => `/api/v1/dns/zones/${id}`,
    {
      invalidateKeys: [['dnsZones'], ['dnsZone']],
      successMessage: 'DNS zone updated',
    },
  );
}

export function useDeleteDNSZone() {
  return useDeleteMutation(
    (id) => `/api/v1/dns/zones/${id}`,
    {
      invalidateKeys: [['dnsZones']],
      successMessage: 'DNS zone deleted',
    },
  );
}

// --- Record queries ---

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

// --- Record mutations ---

export function useCreateDNSRecord(zoneId: string) {
  return useCreateMutation<DNSRecordFormData>(`/api/v1/dns/zones/${zoneId}/records`, {
    invalidateKeys: [['dnsRecords'], ['dnsZone']],
    successMessage: 'DNS record created',
  });
}

export function useUpdateDNSRecord() {
  return useUpdateMutation<Partial<DNSRecordFormData>>(
    (id) => `/api/v1/dns/records/${id}`,
    {
      invalidateKeys: [['dnsRecords'], ['dnsZone']],
      successMessage: 'DNS record updated',
    },
  );
}

export function useDeleteDNSRecord() {
  return useDeleteMutation(
    (id) => `/api/v1/dns/records/${id}`,
    {
      invalidateKeys: [['dnsRecords'], ['dnsZone']],
      successMessage: 'DNS record deleted',
    },
  );
}
