import { useQuery } from '@tanstack/react-query';
import { apiList } from '@/lib/api';
import { transformNodeGroup } from '@/lib/transforms';
import type { ApiNodeGroup } from '@/types/api';
import type { NodeGroup } from '@/types';
import type { ListResult } from '@/lib/api';

export function useNodeGroups(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['nodeGroups', params],
    queryFn: async (): Promise<ListResult<NodeGroup>> => {
      const result = await apiList<ApiNodeGroup>('/api/v1/node-groups', params);
      return {
        ...result,
        items: result.items.map(transformNodeGroup),
      };
    },
  });
}
