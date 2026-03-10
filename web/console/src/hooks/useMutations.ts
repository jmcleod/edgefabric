// Generic mutation hooks wrapping TanStack React Query useMutation with
// query invalidation and toast notifications via sonner.

import { useMutation, useQueryClient, type InvalidateQueryFilters } from '@tanstack/react-query';
import { apiPost, apiPut, apiDelete } from '@/lib/api';
import { toast } from 'sonner';

interface MutationOptions<TResponse> {
  /** Query keys to invalidate on success (e.g. [['tenants'], ['nodes']]) */
  invalidateKeys?: InvalidateQueryFilters['queryKey'][];
  /** Success message shown in toast */
  successMessage?: string;
  /** Called after mutation succeeds */
  onSuccess?: (data: TResponse) => void;
}

/**
 * Generic create mutation — POST to a path with a body, invalidate queries, show toast.
 */
export function useCreateMutation<TBody, TResponse = unknown>(
  path: string,
  opts: MutationOptions<TResponse> = {},
) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: async (body: TBody): Promise<TResponse> => {
      return apiPost<TResponse>(path, body);
    },
    onSuccess: (data) => {
      if (opts.invalidateKeys) {
        opts.invalidateKeys.forEach((key) => qc.invalidateQueries({ queryKey: key }));
      }
      if (opts.successMessage) {
        toast.success(opts.successMessage);
      }
      opts.onSuccess?.(data);
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Operation failed');
    },
  });
}

/**
 * Generic update mutation — PUT to a path with a body.
 * The path function receives the id so callers can build the URL dynamically.
 */
export function useUpdateMutation<TBody, TResponse = unknown>(
  pathFn: (id: string) => string,
  opts: MutationOptions<TResponse> = {},
) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: async ({ id, body }: { id: string; body: TBody }): Promise<TResponse> => {
      return apiPut<TResponse>(pathFn(id), body);
    },
    onSuccess: (data) => {
      if (opts.invalidateKeys) {
        opts.invalidateKeys.forEach((key) => qc.invalidateQueries({ queryKey: key }));
      }
      if (opts.successMessage) {
        toast.success(opts.successMessage);
      }
      opts.onSuccess?.(data);
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Operation failed');
    },
  });
}

/**
 * Generic delete mutation — DELETE a resource by id, invalidate queries, show toast.
 */
export function useDeleteMutation(
  pathFn: (id: string) => string,
  opts: MutationOptions<void> = {},
) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: async (id: string): Promise<void> => {
      return apiDelete(pathFn(id));
    },
    onSuccess: () => {
      if (opts.invalidateKeys) {
        opts.invalidateKeys.forEach((key) => qc.invalidateQueries({ queryKey: key }));
      }
      if (opts.successMessage) {
        toast.success(opts.successMessage);
      }
      opts.onSuccess?.(undefined as void);
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Delete failed');
    },
  });
}

/**
 * Generic action mutation — POST to a path without a body (e.g., node actions like restart).
 */
export function useActionMutation<TResponse = unknown>(
  pathFn: (id: string, action: string) => string,
  opts: MutationOptions<TResponse> = {},
) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: async ({ id, action }: { id: string; action: string }): Promise<TResponse> => {
      return apiPost<TResponse>(pathFn(id, action));
    },
    onSuccess: (data) => {
      if (opts.invalidateKeys) {
        opts.invalidateKeys.forEach((key) => qc.invalidateQueries({ queryKey: key }));
      }
      if (opts.successMessage) {
        toast.success(opts.successMessage);
      }
      opts.onSuccess?.(data);
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Action failed');
    },
  });
}
