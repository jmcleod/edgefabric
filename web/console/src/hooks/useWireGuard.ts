import { useQuery } from '@tanstack/react-query';
import { apiList } from '@/lib/api';
import { transformWireGuardPeer } from '@/lib/transforms';
import type { ApiWireGuardPeer } from '@/types/api';
import type { WireGuardPeer } from '@/types';
import type { ListResult } from '@/lib/api';

export function useWireGuardPeers(ownerId?: string) {
  return useQuery({
    queryKey: ['wireguardPeers', ownerId],
    queryFn: async (): Promise<ListResult<WireGuardPeer>> => {
      const params: Record<string, string | number> = { limit: 100 };
      if (ownerId) params.owner_id = ownerId;
      const result = await apiList<ApiWireGuardPeer>('/api/v1/wireguard/peers', params);
      return {
        ...result,
        items: result.items.map(transformWireGuardPeer),
      };
    },
    enabled: ownerId !== undefined,
  });
}
