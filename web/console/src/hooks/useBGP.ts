import { useQuery } from '@tanstack/react-query';
import { apiList, apiGet } from '@/lib/api';
import { transformBGPSession } from '@/lib/transforms';
import { useCreateMutation, useUpdateMutation, useDeleteMutation } from './useMutations';
import type { ApiBGPSession } from '@/types/api';
import type { BGPPeer } from '@/types';
import type { ListResult } from '@/lib/api';
import type { BGPPeerFormData } from '@/lib/schemas';

export function useBGPSessions(nodeId?: string) {
  return useQuery({
    queryKey: ['bgpSessions', nodeId],
    queryFn: async (): Promise<ListResult<BGPPeer>> => {
      const result = await apiList<ApiBGPSession>(
        `/api/v1/nodes/${nodeId}/bgp-sessions`,
        { limit: 100 },
      );
      return {
        ...result,
        items: result.items.map(transformBGPSession),
      };
    },
    enabled: !!nodeId,
  });
}

export function useBGPSession(id: string | undefined) {
  return useQuery({
    queryKey: ['bgpSession', id],
    queryFn: async () => {
      const api = await apiGet<ApiBGPSession>(`/api/v1/bgp-sessions/${id}`);
      return transformBGPSession(api);
    },
    enabled: !!id,
  });
}

export function useCreateBGPPeer(nodeId: string | undefined) {
  return useCreateMutation<BGPPeerFormData>(
    `/api/v1/nodes/${nodeId}/bgp-sessions`,
    {
      invalidateKeys: [['bgpSessions']],
      successMessage: 'BGP peer created',
    },
  );
}

export function useUpdateBGPPeer() {
  return useUpdateMutation<Partial<BGPPeerFormData>>(
    (id) => `/api/v1/bgp-sessions/${id}`,
    {
      invalidateKeys: [['bgpSessions'], ['bgpSession']],
      successMessage: 'BGP peer updated',
    },
  );
}

export function useDeleteBGPPeer() {
  return useDeleteMutation(
    (id) => `/api/v1/bgp-sessions/${id}`,
    {
      invalidateKeys: [['bgpSessions']],
      successMessage: 'BGP peer deleted',
    },
  );
}
