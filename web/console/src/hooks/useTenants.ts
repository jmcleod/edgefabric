import { useQuery } from '@tanstack/react-query';
import { apiList, apiGet } from '@/lib/api';
import { transformTenant } from '@/lib/transforms';
import { useCreateMutation, useUpdateMutation, useDeleteMutation } from './useMutations';
import type { ApiTenant, ApiNode, ApiDNSZone, ApiCDNSite } from '@/types/api';
import type { Tenant } from '@/types';
import type { ListResult } from '@/lib/api';
import type { TenantFormData } from '@/lib/schemas';

export function useTenants(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['tenants', params],
    queryFn: async (): Promise<ListResult<Tenant>> => {
      const result = await apiList<ApiTenant>('/api/v1/tenants', params);
      return { ...result, items: result.items.map(transformTenant) };
    },
  });
}

export function useTenant(id: string | undefined) {
  return useQuery({
    queryKey: ['tenant', id],
    queryFn: async () => {
      const api = await apiGet<ApiTenant>(`/api/v1/tenants/${id}`);
      return transformTenant(api);
    },
    enabled: !!id,
  });
}

export function useCreateTenant() {
  return useCreateMutation<TenantFormData>('/api/v1/tenants', {
    invalidateKeys: [['tenants']],
    successMessage: 'Tenant created',
  });
}

export function useUpdateTenant() {
  return useUpdateMutation<Partial<TenantFormData>>(
    (id) => `/api/v1/tenants/${id}`,
    {
      invalidateKeys: [['tenants'], ['tenant']],
      successMessage: 'Tenant updated',
    },
  );
}

export function useDeleteTenant() {
  return useDeleteMutation(
    (id) => `/api/v1/tenants/${id}`,
    {
      invalidateKeys: [['tenants']],
      successMessage: 'Tenant deleted',
    },
  );
}

/** Fetch per-tenant resource counts (nodes, DNS zones, CDN sites). */
export function useTenantStats(tenantId: string | undefined) {
  return useQuery({
    queryKey: ['tenant-stats', tenantId],
    queryFn: async () => {
      const [nodes, zones, cdnSites] = await Promise.all([
        apiList<ApiNode>('/api/v1/nodes', { limit: 1 }),
        apiList<ApiDNSZone>(`/api/v1/tenants/${tenantId}/dns/zones`, { limit: 1 }),
        apiList<ApiCDNSite>(`/api/v1/tenants/${tenantId}/cdn/sites`, { limit: 1 }),
      ]);
      return {
        nodeCount: nodes.total,
        zoneCount: zones.total,
        cdnServiceCount: cdnSites.total,
      };
    },
    enabled: !!tenantId,
  });
}
