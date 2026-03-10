import { useQuery } from '@tanstack/react-query';
import { apiList, apiGet } from '@/lib/api';
import { transformRoute } from '@/lib/transforms';
import { useCreateMutation, useUpdateMutation, useDeleteMutation } from './useMutations';
import type { ApiRoute } from '@/types/api';
import type { Route } from '@/types';
import type { ListResult } from '@/lib/api';
import type { RouteFormData } from '@/lib/schemas';

// --- Route queries ---

export function useRoutes(tenantId: string | undefined, params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['routes', tenantId, params],
    queryFn: async (): Promise<ListResult<Route>> => {
      const result = await apiList<ApiRoute>(`/api/v1/tenants/${tenantId}/routes`, params);
      return { ...result, items: result.items.map(transformRoute) };
    },
    enabled: !!tenantId,
  });
}

export function useRoute(routeId: string | undefined) {
  return useQuery({
    queryKey: ['route', routeId],
    queryFn: async () => {
      const api = await apiGet<ApiRoute>(`/api/v1/routes/${routeId}`);
      return transformRoute(api);
    },
    enabled: !!routeId,
  });
}

// --- Route mutations ---

export function useCreateRoute(tenantId: string) {
  return useCreateMutation<RouteFormData>(`/api/v1/tenants/${tenantId}/routes`, {
    invalidateKeys: [['routes']],
    successMessage: 'Route created',
  });
}

export function useUpdateRoute() {
  return useUpdateMutation<Partial<RouteFormData>>(
    (id) => `/api/v1/routes/${id}`,
    {
      invalidateKeys: [['routes'], ['route']],
      successMessage: 'Route updated',
    },
  );
}

export function useDeleteRoute() {
  return useDeleteMutation(
    (id) => `/api/v1/routes/${id}`,
    {
      invalidateKeys: [['routes']],
      successMessage: 'Route deleted',
    },
  );
}
