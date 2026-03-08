import { useQuery } from '@tanstack/react-query';
import { apiList } from '@/lib/api';
import { transformProvisioningJob } from '@/lib/transforms';
import type { ApiProvisioningJob } from '@/types/api';
import type { ProvisioningJob } from '@/types';
import type { ListResult } from '@/lib/api';

export function useProvisioningJobs(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['provisioningJobs', params],
    queryFn: async (): Promise<ListResult<ProvisioningJob>> => {
      const result = await apiList<ApiProvisioningJob>('/api/v1/provisioning/jobs', params);
      return { ...result, items: result.items.map(transformProvisioningJob) };
    },
  });
}
