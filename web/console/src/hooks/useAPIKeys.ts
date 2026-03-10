import { useQuery } from '@tanstack/react-query';
import { apiList } from '@/lib/api';
import { transformAPIKey } from '@/lib/transforms';
import { useCreateMutation, useDeleteMutation } from './useMutations';
import type { ApiAPIKey } from '@/types/api';
import type { APIKey } from '@/types';
import type { ListResult } from '@/lib/api';
import type { APIKeyFormData } from '@/lib/schemas';

export function useAPIKeys(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['apiKeys', params],
    queryFn: async (): Promise<ListResult<APIKey>> => {
      const result = await apiList<ApiAPIKey>('/api/v1/api-keys', params);
      return { ...result, items: result.items.map(transformAPIKey) };
    },
  });
}

export function useCreateAPIKey() {
  return useCreateMutation<APIKeyFormData, { id: string; key: string }>('/api/v1/api-keys', {
    invalidateKeys: [['apiKeys']],
    successMessage: 'API key created',
  });
}

export function useDeleteAPIKey() {
  return useDeleteMutation(
    (id) => `/api/v1/api-keys/${id}`,
    {
      invalidateKeys: [['apiKeys']],
      successMessage: 'API key revoked',
    },
  );
}
