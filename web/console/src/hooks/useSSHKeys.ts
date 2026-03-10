import { useQuery } from '@tanstack/react-query';
import { apiList } from '@/lib/api';
import { transformSSHKey } from '@/lib/transforms';
import { useCreateMutation, useDeleteMutation, useActionMutation } from './useMutations';
import type { ApiSSHKey } from '@/types/api';
import type { SSHKey } from '@/types';
import type { ListResult } from '@/lib/api';
import type { SSHKeyFormData } from '@/lib/schemas';

export function useSSHKeys(params?: { limit?: number; offset?: number }) {
  return useQuery({
    queryKey: ['sshKeys', params],
    queryFn: async (): Promise<ListResult<SSHKey>> => {
      const result = await apiList<ApiSSHKey>('/api/v1/ssh-keys', params);
      return { ...result, items: result.items.map(transformSSHKey) };
    },
  });
}

export function useCreateSSHKey() {
  return useCreateMutation<SSHKeyFormData>('/api/v1/ssh-keys', {
    invalidateKeys: [['sshKeys']],
    successMessage: 'SSH key created',
  });
}

export function useDeleteSSHKey() {
  return useDeleteMutation(
    (id) => `/api/v1/ssh-keys/${id}`,
    {
      invalidateKeys: [['sshKeys']],
      successMessage: 'SSH key deleted',
    },
  );
}

export function useRotateSSHKey() {
  return useActionMutation(
    (id) => `/api/v1/ssh-keys/${id}/rotate`,
    {
      invalidateKeys: [['sshKeys']],
      successMessage: 'SSH key rotated',
    },
  );
}

export function useDeploySSHKey() {
  return useActionMutation(
    (id) => `/api/v1/ssh-keys/${id}/deploy`,
    {
      invalidateKeys: [['sshKeys']],
      successMessage: 'SSH key deployed to nodes',
    },
  );
}
