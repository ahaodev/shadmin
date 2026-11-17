import {
  useMutation,
  useQueryClient,
  type MutationFunction,
} from '@tanstack/react-query'
import { toast } from 'sonner'
import { getErrorMessage } from '@/lib/error'

interface UseCrudMutationOptions<TData, TVariables> {
  mutationFn: MutationFunction<TData, TVariables>
  /** Query keys to invalidate on success. */
  queryKeys: unknown[][] | ((data: TData, variables: TVariables) => unknown[][])
  successMessage: string | ((data: TData, variables: TVariables) => string)
  /** Fallback when the error has no message. */
  errorMessage: string
}

/** Invalidates queries and reports mutation results via toast. */
export function useCrudMutation<TData = unknown, TVariables = void>(
  options: UseCrudMutationOptions<TData, TVariables>
) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: options.mutationFn,
    onSuccess: (data, variables) => {
      const keys =
        typeof options.queryKeys === 'function'
          ? options.queryKeys(data, variables)
          : options.queryKeys
      for (const key of keys) {
        queryClient.invalidateQueries({ queryKey: key })
      }
      const msg =
        typeof options.successMessage === 'function'
          ? options.successMessage(data, variables)
          : options.successMessage
      toast.success(msg)
    },
    onError: (error: unknown) => {
      toast.error(getErrorMessage(error, options.errorMessage))
    },
  })
}
