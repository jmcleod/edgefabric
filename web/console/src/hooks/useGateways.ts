import { useQuery } from '@tanstack/react-query';
import { apiList } from '@/lib/api';
import { transformGateway } from '@/lib/transforms';
import type { ApiGateway } from '@/types/api';
import type { Gateway } from '@/types';
import type { ListResult } from '@/lib/api';

export function useGateways(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['gateways', params],
    queryFn: async (): Promise<ListResult<Gateway>> => {
      const result = await apiList<ApiGateway>('/api/v1/gateways', params);
      return {
        ...result,
        items: result.items.map(transformGateway),
      };
    },
  });
}
