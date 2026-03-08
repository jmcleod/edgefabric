import { useQuery } from '@tanstack/react-query';
import { apiList } from '@/lib/api';
import { transformRoute } from '@/lib/transforms';
import type { ApiRoute } from '@/types/api';
import type { Route } from '@/types';
import type { ListResult } from '@/lib/api';

export function useRoutes(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['routes', params],
    queryFn: async (): Promise<ListResult<Route>> => {
      const result = await apiList<ApiRoute>('/api/v1/routes', params);
      return { ...result, items: result.items.map(transformRoute) };
    },
  });
}
