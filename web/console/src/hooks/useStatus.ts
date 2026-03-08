import { useQuery } from '@tanstack/react-query';
import { apiGet } from '@/lib/api';
import { transformStatus } from '@/lib/transforms';
import type { ApiStatusResponse } from '@/types/api';

export function useStatus() {
  return useQuery({
    queryKey: ['status'],
    queryFn: async () => {
      const api = await apiGet<ApiStatusResponse>('/api/v1/status');
      return { stats: transformStatus(api), raw: api };
    },
    refetchInterval: 30_000, // Refresh every 30s
  });
}
