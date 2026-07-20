import { useQuery } from '@tanstack/react-query'
import { getIdentityProviders } from '@/services/authApi'

const IDENTITY_PROVIDERS_QUERY_KEY = 'identity-providers'

export function useIdentityProviders() {
  return useQuery<string[]>({
    queryKey: [IDENTITY_PROVIDERS_QUERY_KEY],
    queryFn: async () => {
      const response = await getIdentityProviders()

      if (response?.code === 0 && Array.isArray(response.data)) {
        return response.data
      }

      throw new Error('Identity providers unavailable')
    },
    retry: false,
    refetchOnWindowFocus: false,
    staleTime: 5 * 60 * 1000,
  })
}
