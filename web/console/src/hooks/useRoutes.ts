import { useQuery } from '@tanstack/react-query';
import { apiList, apiGet } from '@/lib/api';
import { transformRoute } from '@/lib/transforms';
import type { ApiRoute } from '@/types/api';
import type { Route } from '@/types';
import type { ListResult } from '@/lib/api';

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
