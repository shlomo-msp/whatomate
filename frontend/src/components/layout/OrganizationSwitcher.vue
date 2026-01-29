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
import { Building2, Plus, RefreshCw, Trash2 } from 'lucide-vue-next'
import { organizationsService } from '@/services/api'
import { toast } from 'vue-sonner'

const props = defineProps<{
  collapsed?: boolean
}>()

const organizationsStore = useOrganizationsStore()
const authStore = useAuthStore()
const isRefreshing = ref(false)
const showCreateDialog = ref(false)
const showDeleteDialog = ref(false)
const newOrgName = ref('')
const isCreating = ref(false)
const isDeleting = ref(false)

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
  if (value === '__delete__') {
    openDeleteDialog()
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

const openDeleteDialog = () => {
  if (organizationsStore.organizations.length <= 1) {
    toast.error('Cannot delete the last organization')
    return
  }
  showDeleteDialog.value = true
}

const deleteOrganization = async () => {
  const orgId = organizationsStore.selectedOrgId
  if (!orgId) return

  if (organizationsStore.organizations.length <= 1) {
    toast.error('Cannot delete the last organization')
    return
  }

  isDeleting.value = true
  try {
    await organizationsService.delete(orgId)
    await organizationsStore.fetchOrganizations()
    const nextOrg = organizationsStore.organizations[0]
    if (nextOrg?.id) {
      organizationsStore.selectOrganization(nextOrg.id)
      window.location.reload()
    } else {
      organizationsStore.clearSelection()
    }
    showDeleteDialog.value = false
    toast.success('Organization deleted')
  } catch (err: any) {
    toast.error(err.response?.data?.message || 'Failed to delete organization')
  } finally {
    isDeleting.value = false
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
            <SelectItem
              v-if="organizationsStore.organizations.length > 1"
              value="__delete__"
            >
              <div class="flex items-center gap-2 text-destructive">
                <Trash2 class="h-3.5 w-3.5" />
                <span>Delete organization</span>
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

  <Dialog v-model:open="showDeleteDialog">
    <DialogContent class="sm:max-w-[420px]">
      <DialogHeader>
        <DialogTitle>Delete organization</DialogTitle>
        <DialogDescription>
          This will delete the selected organization and all of its data. This action cannot be undone.
        </DialogDescription>
      </DialogHeader>

      <div class="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
        You must have at least one organization. Deleting the last organization is not allowed.
      </div>

      <DialogFooter class="gap-2">
        <Button variant="outline" @click="showDeleteDialog = false" :disabled="isDeleting">
          Cancel
        </Button>
        <Button variant="destructive" @click="deleteOrganization" :disabled="isDeleting">
          Delete
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
