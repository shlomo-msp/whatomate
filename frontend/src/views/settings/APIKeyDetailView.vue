<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { apiKeysService } from '@/services/api'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
import { formatDateTime } from '@/lib/utils'
import DetailPageLayout from '@/components/shared/DetailPageLayout.vue'
import MetadataPanel from '@/components/shared/MetadataPanel.vue'
import AuditLogPanel from '@/components/shared/AuditLogPanel.vue'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Key, Trash2 } from 'lucide-vue-next'

interface APIKey {
  id: string
  name: string
  key_prefix: string
  last_used_at: string | null
  expires_at: string | null
  is_active: boolean
  created_at: string
  updated_at?: string
}

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const authStore = useAuthStore()

const keyId = computed(() => route.params.id as string)
const apiKey = ref<APIKey | null>(null)
const isLoading = ref(true)
const isNotFound = ref(false)
const deleteDialogOpen = ref(false)

const canDelete = computed(() => authStore.hasPermission('api_keys', 'delete'))

const isExpired = computed(() => {
  if (!apiKey.value?.expires_at) return false
  return new Date(apiKey.value.expires_at) < new Date()
})

const statusVariant = computed(() => {
  if (!apiKey.value) return 'secondary'
  if (!apiKey.value.is_active) return 'secondary'
  if (isExpired.value) return 'destructive'
  return 'default'
})

const statusLabel = computed(() => {
  if (!apiKey.value) return ''
  if (!apiKey.value.is_active) return t('common.inactive', 'Inactive')
  if (isExpired.value) return t('apiKeys.expired', 'Expired')
  return t('common.active', 'Active')
})

const breadcrumbs = computed(() => [
  { label: t('nav.settings'), href: '/settings' },
  { label: t('nav.apiKeys', 'API Keys'), href: '/settings/api-keys' },
  { label: apiKey.value?.name || '' },
])

async function loadApiKey() {
  isLoading.value = true
  isNotFound.value = false
  try {
    const response = await apiKeysService.get(keyId.value)
    const data = (response.data as any).data || response.data
    apiKey.value = data.api_key ?? data
  } catch {
    isNotFound.value = true
  } finally {
    isLoading.value = false
  }
}

async function deleteApiKey() {
  if (!apiKey.value) return
  try {
    await apiKeysService.delete(apiKey.value.id)
    toast.success(t('common.deletedSuccess', { resource: t('resources.apiKey', 'API Key') }))
    router.push('/settings/api-keys')
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedDelete', { resource: t('resources.apiKey', 'API key') })))
  }
  deleteDialogOpen.value = false
}

onMounted(loadApiKey)
</script>

<template>
  <div class="h-full">
    <DetailPageLayout
      :title="apiKey?.name || ''"
      :icon="Key"
      icon-gradient="bg-gradient-to-br from-amber-500 to-orange-600 shadow-amber-500/20"
      back-link="/settings/api-keys"
      :breadcrumbs="breadcrumbs"
      :is-loading="isLoading"
      :is-not-found="isNotFound"
      :not-found-title="$t('apiKeys.notFound', 'API Key not found')"
    >
      <template #actions>
        <div class="flex items-center gap-2">
          <Button
            v-if="canDelete"
            variant="destructive"
            size="sm"
            @click="deleteDialogOpen = true"
          >
            <Trash2 class="h-4 w-4 mr-1" />
            {{ $t('common.delete') }}
          </Button>
        </div>
      </template>

      <Card>
        <CardHeader class="pb-3">
          <div class="flex items-center justify-between">
            <CardTitle class="text-sm font-medium">{{ $t('teams.details', 'Details') }}</CardTitle>
            <div class="flex items-center gap-2">
              <Badge :variant="statusVariant">{{ statusLabel }}</Badge>
            </div>
          </div>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="flex items-center gap-3 p-3 rounded-lg bg-muted/50">
            <div class="h-10 w-10 rounded-full bg-gradient-to-br from-amber-500 to-orange-600 flex items-center justify-center shrink-0">
              <Key class="h-5 w-5 text-white" />
            </div>
            <div class="min-w-0">
              <p class="font-medium truncate">{{ apiKey?.name }}</p>
              <code class="text-xs text-muted-foreground font-mono bg-muted px-1.5 py-0.5 rounded">whm_{{ apiKey?.key_prefix }}...</code>
            </div>
          </div>

          <div class="grid gap-4">
            <div class="space-y-1">
              <p class="text-xs text-muted-foreground">{{ $t('common.name', 'Name') }}</p>
              <p class="text-sm font-medium">{{ apiKey?.name }}</p>
            </div>

            <div class="space-y-1">
              <p class="text-xs text-muted-foreground">{{ $t('apiKeys.keyPrefix', 'Key Prefix') }}</p>
              <code class="text-sm font-mono bg-muted px-2 py-1 rounded inline-block">whm_{{ apiKey?.key_prefix }}...</code>
            </div>

            <div class="space-y-1">
              <p class="text-xs text-muted-foreground">{{ $t('common.status', 'Status') }}</p>
              <div class="flex items-center gap-2">
                <Badge :variant="statusVariant">{{ statusLabel }}</Badge>
                <Badge v-if="isExpired" variant="destructive">{{ $t('apiKeys.expired', 'Expired') }}</Badge>
              </div>
            </div>

            <div class="space-y-1">
              <p class="text-xs text-muted-foreground">{{ $t('common.createdAt', 'Created') }}</p>
              <p class="text-sm">{{ apiKey?.created_at ? formatDateTime(apiKey.created_at) : '—' }}</p>
            </div>

            <div class="space-y-1">
              <p class="text-xs text-muted-foreground">{{ $t('apiKeys.lastUsedAt', 'Last Used') }}</p>
              <p class="text-sm">{{ apiKey?.last_used_at ? formatDateTime(apiKey.last_used_at) : '—' }}</p>
            </div>

            <div class="space-y-1">
              <p class="text-xs text-muted-foreground">{{ $t('apiKeys.expiresAt', 'Expires') }}</p>
              <div class="flex items-center gap-2">
                <p class="text-sm">{{ apiKey?.expires_at ? formatDateTime(apiKey.expires_at) : $t('apiKeys.never', 'Never') }}</p>
                <Badge v-if="isExpired" variant="destructive" class="text-xs">{{ $t('apiKeys.expired', 'Expired') }}</Badge>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <AuditLogPanel
        v-if="apiKey"
        resource-type="api_key"
        :resource-id="apiKey.id"
      />

      <template #sidebar>
        <MetadataPanel
          :created-at="apiKey?.created_at"
          :updated-at="apiKey?.updated_at"
        />
      </template>
    </DetailPageLayout>

    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ $t('apiKeys.deleteKey', 'Delete API Key') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ $t('apiKeys.deleteWarning', 'Are you sure you want to delete this API key? Any applications using this key will immediately lose access. This action cannot be undone.') }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ $t('common.cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="deleteApiKey">{{ $t('common.delete') }}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
