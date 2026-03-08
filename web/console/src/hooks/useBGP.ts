import { useQuery } from '@tanstack/react-query';
import { apiList } from '@/lib/api';
import { transformBGPSession } from '@/lib/transforms';
import type { ApiBGPSession } from '@/types/api';
import type { BGPPeer } from '@/types';
import type { ListResult } from '@/lib/api';

export function useBGPSessions(nodeId?: string) {
  return useQuery({
    queryKey: ['bgpSessions', nodeId],
    queryFn: async (): Promise<ListResult<BGPPeer>> => {
      const params: Record<string, string | number> = { limit: 100 };
      if (nodeId) params.node_id = nodeId;
      const result = await apiList<ApiBGPSession>('/api/v1/bgp-sessions', params);
      return {
        ...result,
        items: result.items.map(transformBGPSession),
      };
    },
    enabled: nodeId !== undefined,
  });
}
