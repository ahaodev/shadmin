import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { OAuthCallback } from '@/features/auth/oauth-callback'

const searchSchema = z.object({
  code: z.string().optional(),
  error: z.string().optional(),
})

export const Route = createFileRoute('/(auth)/oauth-callback')({
  component: OAuthCallback,
  validateSearch: searchSchema,
})
