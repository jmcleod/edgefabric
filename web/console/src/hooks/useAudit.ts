import { useQuery } from '@tanstack/react-query';
import { apiList } from '@/lib/api';
import { transformAuditEvent } from '@/lib/transforms';
import type { ApiAuditEvent } from '@/types/api';
import type { AuditLogEntry } from '@/types';
import type { ListResult } from '@/lib/api';

export function useAuditLogs(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['auditLogs', params],
    queryFn: async (): Promise<ListResult<AuditLogEntry>> => {
      const result = await apiList<ApiAuditEvent>('/api/v1/audit', params);
      return { ...result, items: result.items.map(transformAuditEvent) };
    },
  });
}
