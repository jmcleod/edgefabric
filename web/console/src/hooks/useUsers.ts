import { useQuery } from '@tanstack/react-query';
import { apiList, apiGet } from '@/lib/api';
import { transformUser } from '@/lib/transforms';
import { useCreateMutation, useUpdateMutation, useDeleteMutation } from './useMutations';
import type { ApiUser } from '@/types/api';
import type { User } from '@/types';
import type { ListResult } from '@/lib/api';
import type { UserFormData } from '@/lib/schemas';

export function useUsers(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['users', params],
    queryFn: async (): Promise<ListResult<User>> => {
      const result = await apiList<ApiUser>('/api/v1/users', params);
      return { ...result, items: result.items.map(transformUser) };
    },
  });
}

export function useUser(id: string | undefined) {
  return useQuery({
    queryKey: ['user', id],
    queryFn: async () => {
      const api = await apiGet<ApiUser>(`/api/v1/users/${id}`);
      return transformUser(api);
    },
    enabled: !!id,
  });
}

export function useCreateUser() {
  return useCreateMutation<UserFormData>('/api/v1/users', {
    invalidateKeys: [['users']],
    successMessage: 'User created',
  });
}

export function useUpdateUser() {
  return useUpdateMutation<Partial<UserFormData>>(
    (id) => `/api/v1/users/${id}`,
    {
      invalidateKeys: [['users'], ['user']],
      successMessage: 'User updated',
    },
  );
}

export function useDeleteUser() {
  return useDeleteMutation(
    (id) => `/api/v1/users/${id}`,
    {
      invalidateKeys: [['users']],
      successMessage: 'User deleted',
    },
  );
}
