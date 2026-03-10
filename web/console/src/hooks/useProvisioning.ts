import { useQuery } from '@tanstack/react-query';
import { apiList, apiGet } from '@/lib/api';
import { transformProvisioningJob } from '@/lib/transforms';
import type { ApiProvisioningJob } from '@/types/api';
import type { ProvisioningJob } from '@/types';
import type { ListResult } from '@/lib/api';

export function useProvisioningJobs(nodeId: string | undefined, params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['provisioningJobs', nodeId, params],
    queryFn: async (): Promise<ListResult<ProvisioningJob>> => {
      const result = await apiList<ApiProvisioningJob>(`/api/v1/nodes/${nodeId}/jobs`, params);
      return { ...result, items: result.items.map(transformProvisioningJob) };
    },
    enabled: !!nodeId,
  });
}

export function useProvisioningJob(jobId: string | undefined) {
  return useQuery({
    queryKey: ['provisioningJob', jobId],
    queryFn: async () => {
      const api = await apiGet<ApiProvisioningJob>(`/api/v1/provisioning/jobs/${jobId}`);
      return transformProvisioningJob(api);
    },
    enabled: !!jobId,
  });
}
