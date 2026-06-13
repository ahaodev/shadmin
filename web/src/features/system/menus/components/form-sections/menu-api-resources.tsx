import { useEffect } from 'react'
import type { UseFormReturn } from 'react-hook-form'
import type { ApiResource } from '@/types/api-resource'
import { X } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { METHOD_COLORS } from '@/features/system/api-resources/constants/api-constants'
import { useApiResources } from '@/features/system/api-resources/hooks/use-api-resources'
import type { CreateMenuFormData } from '../../schemas/menu-form-schema'
import { useMenus } from '../menus-provider'
import { ApiResourceSelector } from './api-resource-selector'

interface MenuApiResourcesProps {
  form: UseFormReturn<CreateMenuFormData>
  isEditMode?: boolean
}

export function MenuApiResources({
  form,
  isEditMode = false,
}: MenuApiResourcesProps) {
  const { currentRow } = useMenus()

  const selectedPermissions = form.watch('apiResources') ?? []

  useEffect(() => {
    if (!isEditMode) return
    const current = form.getValues('apiResources') ?? []
    if (current.length > 0) return
    const seed = currentRow?.apiResources ?? []
    if (seed.length === 0) return
    form.setValue('apiResources', seed, {
      shouldDirty: false,
      shouldValidate: false,
    })
  }, [currentRow, isEditMode, form])

  // Fetch API resources from backend
  const { data: apiResourcesResult, isLoading } = useApiResources({
    page: 1,
    page_size: 1000,
  })
  const apiResources = apiResourcesResult?.data || []

  const writeSelection = (next: string[]) => {
    form.setValue('apiResources', next, {
      shouldDirty: true,
      shouldValidate: false,
    })
  }

  const handlePermissionToggle = (permissionId: string, checked: boolean) => {
    if (checked) {
      if (selectedPermissions.includes(permissionId)) return
      writeSelection([...selectedPermissions, permissionId])
    } else {
      writeSelection(selectedPermissions.filter((id) => id !== permissionId))
    }
  }

  const handleResourceSelect = (resource: ApiResource, checked: boolean) => {
    // Use method:path format to match backend expectations
    const resourceIdentifier = `${resource.method}:${resource.path}`
    handlePermissionToggle(resourceIdentifier, checked)
  }

  const isResourceSelected = (resource: ApiResource) => {
    const methodPathFormat = `${resource.method}:${resource.path}`
    return (
      selectedPermissions.includes(resource.id) ||
      selectedPermissions.includes(methodPathFormat)
    )
  }

  const handleRemovePermission = (permissionId: string) => {
    writeSelection(selectedPermissions.filter((id) => id !== permissionId))
  }

  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between'>
        <div className='text-sm font-medium'>
          API资源{' '}
          {selectedPermissions.length > 0 && `(${selectedPermissions.length})`}
        </div>
        <ApiResourceSelector
          apiResources={apiResources}
          isLoading={isLoading}
          selectedApiResources={selectedPermissions}
          onResourceSelect={handleResourceSelect}
          isResourceSelected={isResourceSelected}
        />
      </div>

      {/* Selected permissions display */}
      {selectedPermissions.length > 0 && (
        <div className='flex flex-wrap gap-2'>
          {selectedPermissions.map((resourceId) => {
            // Find the corresponding API resource for this ID
            // Support both direct ID matching and method:path format matching
            const matchingResource = apiResources.find((resource) => {
              // Direct ID match
              if (resource.id === resourceId) return true

              // Method:path format match (e.g., "POST:/api/v1/system/menu")
              const methodPathFormat = `${resource.method}:${resource.path}`
              if (methodPathFormat === resourceId) return true

              return false
            })

            return matchingResource ? (
              <Badge
                key={resourceId}
                variant='secondary'
                className='flex items-center space-x-1 pr-1'
              >
                <Badge
                  className={`mr-1 text-xs ${METHOD_COLORS[matchingResource.method as keyof typeof METHOD_COLORS] || 'bg-gray-100 text-gray-800'}`}
                >
                  {matchingResource.method}
                </Badge>
                <span>{matchingResource.path}</span>
                <Button
                  variant='ghost'
                  size='sm'
                  className='h-auto w-auto p-0.5 hover:bg-transparent'
                  onClick={() => handleRemovePermission(resourceId)}
                >
                  <X className='h-3 w-3' />
                </Button>
              </Badge>
            ) : (
              <Badge
                key={resourceId}
                variant='outline'
                className='text-muted-foreground flex items-center space-x-1 pr-1'
              >
                <span>{resourceId}</span>
                <Button
                  variant='ghost'
                  size='sm'
                  className='h-auto w-auto p-0.5 hover:bg-transparent'
                  onClick={() => handleRemovePermission(resourceId)}
                >
                  <X className='h-3 w-3' />
                </Button>
              </Badge>
            )
          })}
        </div>
      )}
    </div>
  )
}
