import { useQuery } from '@tanstack/react-query';
import { apiList, apiGet } from '@/lib/api';
import { transformIPAllocation } from '@/lib/transforms';
import { useCreateMutation, useUpdateMutation, useDeleteMutation } from './useMutations';
import type { ApiIPAllocation } from '@/types/api';
import type { IPAllocation } from '@/types';
import type { ListResult } from '@/lib/api';
import type { IPAllocationFormData } from '@/lib/schemas';

export function useIPAllocations(tenantId: string | undefined, params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['ipAllocations', tenantId, params],
    queryFn: async (): Promise<ListResult<IPAllocation>> => {
      const result = await apiList<ApiIPAllocation>(`/api/v1/tenants/${tenantId}/ip-allocations`, params);
      return { ...result, items: result.items.map(transformIPAllocation) };
    },
    enabled: !!tenantId,
  });
}

export function useIPAllocation(id: string | undefined) {
  return useQuery({
    queryKey: ['ipAllocation', id],
    queryFn: async () => {
      const api = await apiGet<ApiIPAllocation>(`/api/v1/ip-allocations/${id}`);
      return transformIPAllocation(api);
    },
    enabled: !!id,
  });
}

export function useCreateIPAllocation(tenantId: string | undefined) {
  return useCreateMutation<IPAllocationFormData>(
    `/api/v1/tenants/${tenantId}/ip-allocations`,
    {
      invalidateKeys: [['ipAllocations']],
      successMessage: 'IP allocation created',
    },
  );
}

export function useDeleteIPAllocation() {
  return useDeleteMutation(
    (id) => `/api/v1/ip-allocations/${id}`,
    {
      invalidateKeys: [['ipAllocations']],
      successMessage: 'IP allocation deleted',
    },
  );
}
