import { useQuery } from '@tanstack/react-query';
import { apiList } from '@/lib/api';
import { transformAPIKey } from '@/lib/transforms';
import type { ApiAPIKey } from '@/types/api';
import type { APIKey } from '@/types';
import type { ListResult } from '@/lib/api';

export function useAPIKeys(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['apiKeys', params],
    queryFn: async (): Promise<ListResult<APIKey>> => {
      const result = await apiList<ApiAPIKey>('/api/v1/api-keys', params);
      return { ...result, items: result.items.map(transformAPIKey) };
    },
  });
}
