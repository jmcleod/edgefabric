import { useQuery } from '@tanstack/react-query';
import { apiGet, apiList } from '@/lib/api';
import { transformNode } from '@/lib/transforms';
import type { ApiNode } from '@/types/api';
import type { Node } from '@/types';
import type { ListResult } from '@/lib/api';

export function useNodes(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['nodes', params],
    queryFn: async (): Promise<ListResult<Node>> => {
      const result = await apiList<ApiNode>('/api/v1/nodes', params);
      return {
        ...result,
        items: result.items.map(transformNode),
      };
    },
  });
}

export function useNode(id: string | undefined) {
  return useQuery({
    queryKey: ['node', id],
    queryFn: async () => {
      const api = await apiGet<ApiNode>(`/api/v1/nodes/${id}`);
      return transformNode(api);
    },
    enabled: !!id,
  });
}
