import { useQuery } from '@tanstack/react-query';
import { apiList, apiGet, apiPost } from '@/lib/api';
import { transformCDNSite, transformCDNOrigin } from '@/lib/transforms';
import { useCreateMutation, useUpdateMutation, useDeleteMutation } from './useMutations';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type { ApiCDNSite, ApiCDNOrigin } from '@/types/api';
import type { CDNService, CDNOrigin } from '@/types';
import type { ListResult } from '@/lib/api';
import type { CDNSiteFormData, CDNOriginFormData, CachePurgeFormData } from '@/lib/schemas';

// --- Site queries ---

export function useCDNSites(tenantId: string | undefined, params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['cdnSites', tenantId, params],
    queryFn: async (): Promise<ListResult<CDNService>> => {
      const result = await apiList<ApiCDNSite>(`/api/v1/tenants/${tenantId}/cdn/sites`, params);
      return { ...result, items: result.items.map(transformCDNSite) };
    },
    enabled: !!tenantId,
  });
}

export function useCDNSite(siteId: string | undefined) {
  return useQuery({
    queryKey: ['cdnSite', siteId],
    queryFn: async () => {
      const api = await apiGet<ApiCDNSite>(`/api/v1/cdn/sites/${siteId}`);
      return transformCDNSite(api);
    },
    enabled: !!siteId,
  });
}

// --- Site mutations ---

export function useCreateCDNSite(tenantId: string) {
  return useCreateMutation<CDNSiteFormData>(`/api/v1/tenants/${tenantId}/cdn/sites`, {
    invalidateKeys: [['cdnSites']],
    successMessage: 'CDN site created',
  });
}

export function useUpdateCDNSite() {
  return useUpdateMutation<Partial<CDNSiteFormData>>(
    (id) => `/api/v1/cdn/sites/${id}`,
    {
      invalidateKeys: [['cdnSites'], ['cdnSite']],
      successMessage: 'CDN site updated',
    },
  );
}

export function useDeleteCDNSite() {
  return useDeleteMutation(
    (id) => `/api/v1/cdn/sites/${id}`,
    {
      invalidateKeys: [['cdnSites']],
      successMessage: 'CDN site deleted',
    },
  );
}

// --- Origin queries ---

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

// --- Origin mutations ---

export function useCreateCDNOrigin(siteId: string) {
  return useCreateMutation<CDNOriginFormData>(`/api/v1/cdn/sites/${siteId}/origins`, {
    invalidateKeys: [['cdnOrigins'], ['cdnSite']],
    successMessage: 'Origin added',
  });
}

export function useUpdateCDNOrigin() {
  return useUpdateMutation<Partial<CDNOriginFormData>>(
    (id) => `/api/v1/cdn/origins/${id}`,
    {
      invalidateKeys: [['cdnOrigins'], ['cdnSite']],
      successMessage: 'Origin updated',
    },
  );
}

export function useDeleteCDNOrigin() {
  return useDeleteMutation(
    (id) => `/api/v1/cdn/origins/${id}`,
    {
      invalidateKeys: [['cdnOrigins'], ['cdnSite']],
      successMessage: 'Origin deleted',
    },
  );
}

// --- Cache purge ---

export function usePurgeCache(siteId: string) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: async (body: CachePurgeFormData) => {
      return apiPost(`/api/v1/cdn/sites/${siteId}/purge`, {
        paths: body.paths.split('\n').map((p) => p.trim()).filter(Boolean),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['cdnSite'] });
      toast.success('Cache purge initiated');
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Cache purge failed');
    },
  });
}
