import { useQuery } from '@tanstack/react-query';
import { apiList, apiGet } from '@/lib/api';
import { transformGateway } from '@/lib/transforms';
import { useCreateMutation, useUpdateMutation, useDeleteMutation } from './useMutations';
import type { ApiGateway } from '@/types/api';
import type { Gateway } from '@/types';
import type { ListResult } from '@/lib/api';
import type { GatewayFormData } from '@/lib/schemas';

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

export function useGateway(id: string | undefined) {
  return useQuery({
    queryKey: ['gateway', id],
    queryFn: async () => {
      const api = await apiGet<ApiGateway>(`/api/v1/gateways/${id}`);
      return transformGateway(api);
    },
    enabled: !!id,
  });
}

export function useCreateGateway() {
  return useCreateMutation<GatewayFormData>('/api/v1/gateways', {
    invalidateKeys: [['gateways']],
    successMessage: 'Gateway created',
  });
}

export function useUpdateGateway() {
  return useUpdateMutation<Partial<GatewayFormData>>(
    (id) => `/api/v1/gateways/${id}`,
    {
      invalidateKeys: [['gateways'], ['gateway']],
      successMessage: 'Gateway updated',
    },
  );
}

export function useDeleteGateway() {
  return useDeleteMutation(
    (id) => `/api/v1/gateways/${id}`,
    {
      invalidateKeys: [['gateways']],
      successMessage: 'Gateway deleted',
    },
  );
}
