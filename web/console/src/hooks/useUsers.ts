import { useQuery } from '@tanstack/react-query';
import { apiList } from '@/lib/api';
import { transformUser } from '@/lib/transforms';
import type { ApiUser } from '@/types/api';
import type { User } from '@/types';
import type { ListResult } from '@/lib/api';

export function useUsers(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['users', params],
    queryFn: async (): Promise<ListResult<User>> => {
      const result = await apiList<ApiUser>('/api/v1/users', params);
      return { ...result, items: result.items.map(transformUser) };
    },
  });
}
