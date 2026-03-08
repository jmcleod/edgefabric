import { useQuery } from '@tanstack/react-query';
import { apiList } from '@/lib/api';
import { transformTenant } from '@/lib/transforms';
import type { ApiTenant } from '@/types/api';
import type { Tenant } from '@/types';
import type { ListResult } from '@/lib/api';

export function useTenants(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['tenants', params],
    queryFn: async (): Promise<ListResult<Tenant>> => {
      const result = await apiList<ApiTenant>('/api/v1/tenants', params);
      return { ...result, items: result.items.map(transformTenant) };
    },
  });
}
