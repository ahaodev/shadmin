import { createFileRoute } from '@tanstack/react-router'
import { Departments } from '@/features/system/departments'

export const Route = createFileRoute('/_authenticated/system/departments')({
  component: Departments,
})
