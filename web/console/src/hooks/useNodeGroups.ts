import { useQuery } from '@tanstack/react-query';
import { apiList, apiGet } from '@/lib/api';
import { transformNodeGroup } from '@/lib/transforms';
import { useCreateMutation, useDeleteMutation } from './useMutations';
import type { ApiNodeGroup } from '@/types/api';
import type { NodeGroup } from '@/types';
import type { ListResult } from '@/lib/api';
import type { NodeGroupFormData } from '@/lib/schemas';

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

export function useNodeGroup(id: string | undefined) {
  return useQuery({
    queryKey: ['nodeGroup', id],
    queryFn: async () => {
      const api = await apiGet<ApiNodeGroup>(`/api/v1/node-groups/${id}`);
      return transformNodeGroup(api);
    },
    enabled: !!id,
  });
}

export function useCreateNodeGroup() {
  return useCreateMutation<NodeGroupFormData>('/api/v1/node-groups', {
    invalidateKeys: [['nodeGroups']],
    successMessage: 'Node group created',
  });
}

export function useDeleteNodeGroup() {
  return useDeleteMutation(
    (id) => `/api/v1/node-groups/${id}`,
    {
      invalidateKeys: [['nodeGroups']],
      successMessage: 'Node group deleted',
    },
  );
}
