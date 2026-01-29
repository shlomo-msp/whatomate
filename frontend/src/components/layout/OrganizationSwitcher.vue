<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useOrganizationsStore } from '@/stores/organizations'
import { useAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Building2, Plus, RefreshCw } from 'lucide-vue-next'
import { organizationsService } from '@/services/api'
import { toast } from 'vue-sonner'

const props = defineProps<{
  collapsed?: boolean
}>()

const organizationsStore = useOrganizationsStore()
const authStore = useAuthStore()
const isRefreshing = ref(false)
const showCreateDialog = ref(false)
const newOrgName = ref('')
const isCreating = ref(false)

// Only show for super admins
const isSuperAdmin = () => authStore.user?.is_super_admin || false

onMounted(async () => {
  if (isSuperAdmin()) {
    organizationsStore.init()
    await organizationsStore.fetchOrganizations()

    // If no org selected, default to user's own org
    if (!organizationsStore.selectedOrgId && authStore.user?.organization_id) {
      organizationsStore.selectOrganization(authStore.user.organization_id)
    }
  }
})

// Watch for auth changes
watch(() => authStore.user?.is_super_admin, async (isSuperAdmin) => {
  if (isSuperAdmin) {
    organizationsStore.init()
    await organizationsStore.fetchOrganizations()
  } else {
    organizationsStore.reset()
  }
})

const handleOrgChange = (value: string | number | bigint | Record<string, any> | null) => {
  if (!value || typeof value !== 'string') return
  if (value === '__create__') {
    showCreateDialog.value = true
    return
  }
  organizationsStore.selectOrganization(value)
  // Reload the page to refresh data with new org context
  window.location.reload()
}

const refreshOrgs = async () => {
  isRefreshing.value = true
  await organizationsStore.fetchOrganizations()
  isRefreshing.value = false
}

const createOrganization = async () => {
  const name = newOrgName.value.trim()
  if (!name) {
    toast.error('Organization name is required')
    return
  }

  isCreating.value = true
  try {
    const response = await organizationsService.create({ name })
    const org = (response.data as any).data?.organization || response.data?.organization
    await organizationsStore.fetchOrganizations()
    if (org?.id) {
      organizationsStore.selectOrganization(org.id)
      window.location.reload()
    }
    newOrgName.value = ''
    showCreateDialog.value = false
    toast.success('Organization created')
  } catch (err: any) {
    toast.error(err.response?.data?.message || 'Failed to create organization')
  } finally {
    isCreating.value = false
  }
}
</script>

<template>
  <div v-if="isSuperAdmin()" class="px-2 py-2 border-b">
    <div v-if="!collapsed" class="space-y-1">
      <div class="flex items-center justify-between">
        <span class="text-[11px] font-medium text-muted-foreground uppercase tracking-wide px-1">
          Organization
        </span>
        <Button
          variant="ghost"
          size="icon"
          class="h-5 w-5"
          @click="refreshOrgs"
          :disabled="isRefreshing"
        >
          <RefreshCw :class="['h-3 w-3', isRefreshing && 'animate-spin']" />
        </Button>
      </div>
      <div v-if="organizationsStore.organizations.length > 0" class="space-y-2">
        <Select
          :model-value="organizationsStore.selectedOrgId || ''"
          @update:model-value="handleOrgChange"
        >
          <SelectTrigger class="h-8 text-[13px]">
            <SelectValue placeholder="Select organization" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem
              v-for="org in organizationsStore.organizations"
              :key="org.id"
              :value="org.id"
            >
              <div class="flex items-center gap-2">
                <Building2 class="h-3.5 w-3.5 text-muted-foreground" />
                <span>{{ org.name }}</span>
              </div>
            </SelectItem>
            <SelectItem value="__create__">
              <div class="flex items-center gap-2">
                <Plus class="h-3.5 w-3.5 text-muted-foreground" />
                <span>Add organization</span>
              </div>
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div v-else-if="organizationsStore.loading" class="text-[12px] text-muted-foreground px-1">
        Loading...
      </div>
      <div v-else-if="organizationsStore.error" class="text-[12px] text-destructive px-1">
        {{ organizationsStore.error }}
      </div>
      <div v-else class="text-[12px] text-muted-foreground px-1">
        No organizations found
      </div>
    </div>

    <!-- Collapsed view - just show icon with selected org initial -->
    <div v-else class="flex justify-center">
      <Button
        variant="ghost"
        size="icon"
        class="h-8 w-8"
        :title="organizationsStore.selectedOrganization?.name || 'All Organizations'"
      >
        <Building2 class="h-4 w-4" />
      </Button>
    </div>
  </div>

  <Dialog v-model:open="showCreateDialog">
    <DialogContent class="sm:max-w-[420px]">
      <DialogHeader>
        <DialogTitle>Create organization</DialogTitle>
        <DialogDescription>
          Add a new organization to manage within this deployment.
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-2">
        <Label for="org-name">Organization name</Label>
        <Input
          id="org-name"
          v-model="newOrgName"
          placeholder="Acme Inc."
          :disabled="isCreating"
        />
      </div>

      <DialogFooter class="gap-2">
        <Button variant="outline" @click="showCreateDialog = false" :disabled="isCreating">
          Cancel
        </Button>
        <Button @click="createOrganization" :disabled="isCreating">
          Create
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

</template>
