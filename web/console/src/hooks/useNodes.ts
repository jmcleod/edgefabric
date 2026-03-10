import { useQuery } from '@tanstack/react-query';
import { apiGet, apiList } from '@/lib/api';
import { transformNode } from '@/lib/transforms';
import { useCreateMutation, useUpdateMutation, useDeleteMutation, useActionMutation } from './useMutations';
import type { ApiNode } from '@/types/api';
import type { Node } from '@/types';
import type { ListResult } from '@/lib/api';
import type { NodeFormData } from '@/lib/schemas';

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

export function useCreateNode() {
  return useCreateMutation<NodeFormData>('/api/v1/nodes', {
    invalidateKeys: [['nodes']],
    successMessage: 'Node created',
  });
}

export function useUpdateNode() {
  return useUpdateMutation<Partial<NodeFormData>>(
    (id) => `/api/v1/nodes/${id}`,
    {
      invalidateKeys: [['nodes'], ['node']],
      successMessage: 'Node updated',
    },
  );
}

export function useDeleteNode() {
  return useDeleteMutation(
    (id) => `/api/v1/nodes/${id}`,
    {
      invalidateKeys: [['nodes']],
      successMessage: 'Node deleted',
    },
  );
}

export type NodeAction = 'enroll' | 'start' | 'stop' | 'restart' | 'upgrade' | 'reprovision' | 'decommission';

export function useNodeAction() {
  return useActionMutation(
    (id, action) => `/api/v1/nodes/${id}/${action}`,
    {
      invalidateKeys: [['nodes'], ['node'], ['provisioningJobs']],
      successMessage: 'Node action initiated',
    },
  );
}
