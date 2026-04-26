<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { toast } from 'vue-sonner'
import { User, Eye, EyeOff, Loader2 } from 'lucide-vue-next'
import { usersService } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { PageHeader } from '@/components/shared'
import { getErrorMessage } from '@/lib/api-utils'

const { t } = useI18n()
const authStore = useAuthStore()
const isChangingPassword = ref(false)
const isSettingUpTwoFA = ref(false)
const isVerifyingTwoFA = ref(false)
const isDisablingTwoFA = ref(false)
const isResettingTwoFA = ref(false)
const showCurrentPassword = ref(false)
const showNewPassword = ref(false)
const showConfirmPassword = ref(false)
const showDisablePassword = ref(false)
const showResetPassword = ref(false)
const setupDialogOpen = ref(false)
const resetDialogOpen = ref(false)
const disableDialogOpen = ref(false)

const passwordForm = ref({
  current_password: '',
  new_password: '',
  confirm_password: ''
})

const twoFASetup = ref({
  secret: '',
  otpauth_url: '',
  qr_code: '',
  code: ''
})

const twoFADisable = ref({
  current_password: ''
})

const twoFAReset = ref({
  current_password: '',
  secret: '',
  otpauth_url: '',
  qr_code: '',
  code: ''
})

async function changePassword() {
  // Validate passwords match
  if (passwordForm.value.new_password !== passwordForm.value.confirm_password) {
    toast.error(t('profile.passwordMismatch'))
    return
  }

  // Validate password length
  if (passwordForm.value.new_password.length < 6) {
    toast.error(t('profile.passwordTooShort'))
    return
  }

  isChangingPassword.value = true
  try {
    await usersService.changePassword({
      current_password: passwordForm.value.current_password,
      new_password: passwordForm.value.new_password
    })
    toast.success(t('profile.passwordChanged'))
    // Clear the form
    passwordForm.value = {
      current_password: '',
      new_password: '',
      confirm_password: ''
    }
  } catch (error: any) {
    toast.error(getErrorMessage(error, t('profile.passwordChangeFailed')))
  } finally {
    isChangingPassword.value = false
  }
}

async function setupTwoFA() {
  isSettingUpTwoFA.value = true
  try {
    const response = await usersService.setupTwoFA()
    const data = response.data.data
    twoFASetup.value.secret = data.secret
    twoFASetup.value.otpauth_url = data.otpauth_url
    twoFASetup.value.qr_code = data.qr_code
    setupDialogOpen.value = true
    toast.success('Scan the QR code and enter the verification code')
  } catch (error: any) {
    toast.error(getErrorMessage(error, 'Failed to start 2FA setup'))
  } finally {
    isSettingUpTwoFA.value = false
  }
}

async function verifyTwoFA() {
  if (!twoFASetup.value.code) {
    toast.error('Enter the verification code')
    return
  }

  isVerifyingTwoFA.value = true
  try {
    await usersService.verifyTwoFA(twoFASetup.value.code)
    toast.success('Two-factor authentication enabled')
    setupDialogOpen.value = false
    twoFASetup.value = { secret: '', otpauth_url: '', qr_code: '', code: '' }
    await authStore.refreshUserData()
  } catch (error: any) {
    toast.error(getErrorMessage(error, 'Failed to verify 2FA'))
  } finally {
    isVerifyingTwoFA.value = false
  }
}

async function disableTwoFA() {
  if (!twoFADisable.value.current_password) {
    toast.error('Enter your current password')
    return
  }

  isDisablingTwoFA.value = true
  try {
    await usersService.disableTwoFA(twoFADisable.value.current_password)
    toast.success('Two-factor authentication disabled')
    twoFADisable.value = { current_password: '' }
    disableDialogOpen.value = false
    await authStore.refreshUserData()
  } catch (error: any) {
    toast.error(getErrorMessage(error, 'Failed to disable 2FA'))
  } finally {
    isDisablingTwoFA.value = false
  }
}

async function resetTwoFA() {
  if (!twoFAReset.value.current_password) {
    toast.error('Enter your current password')
    return
  }

  isResettingTwoFA.value = true
  try {
    const response = await usersService.resetTwoFA(twoFAReset.value.current_password)
    const data = response.data.data
    twoFAReset.value.secret = data.secret
    twoFAReset.value.otpauth_url = data.otpauth_url
    twoFAReset.value.qr_code = data.qr_code
  } catch (error: any) {
    toast.error(getErrorMessage(error, 'Failed to reset 2FA'))
  } finally {
    isResettingTwoFA.value = false
  }
}

async function verifyResetTwoFA() {
  if (!twoFAReset.value.code) {
    toast.error('Enter the verification code')
    return
  }

  isVerifyingTwoFA.value = true
  try {
    await usersService.verifyTwoFA(twoFAReset.value.code)
    toast.success('Two-factor authentication reset')
    resetDialogOpen.value = false
    twoFAReset.value = { current_password: '', secret: '', otpauth_url: '', qr_code: '', code: '' }
    await authStore.refreshUserData()
  } catch (error: any) {
    toast.error(getErrorMessage(error, 'Failed to verify 2FA'))
  } finally {
    isVerifyingTwoFA.value = false
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader
      :title="$t('profile.title')"
      :description="$t('profile.description')"
      :icon="User"
      icon-gradient="bg-gradient-to-br from-gray-500 to-gray-600 shadow-gray-500/20"
    />

    <!-- Content -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-6 max-w-2xl mx-auto">
        <!-- User Info -->
        <Card>
          <CardHeader>
            <CardTitle>{{ $t('profile.accountInfo') }}</CardTitle>
            <CardDescription>{{ $t('profile.accountInfoDesc') }}</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="grid grid-cols-2 gap-4">
              <div>
                <Label class="text-muted-foreground">{{ $t('common.name') }}</Label>
                <p class="font-medium">{{ authStore.user?.full_name }}</p>
              </div>
              <div>
                <Label class="text-muted-foreground">{{ $t('common.email') }}</Label>
                <p class="font-medium">{{ authStore.user?.email }}</p>
              </div>
              <div>
                <Label class="text-muted-foreground">{{ $t('users.role') }}</Label>
                <p class="font-medium capitalize">{{ authStore.user?.role?.name }}</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- Change Password -->
        <Card>
          <CardHeader>
            <CardTitle>{{ $t('profile.changePassword') }}</CardTitle>
            <CardDescription>{{ $t('profile.changePasswordDesc') }}</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="space-y-2">
              <Label for="current_password">{{ $t('profile.currentPassword') }}</Label>
              <div class="relative">
                <Input
                  id="current_password"
                  v-model="passwordForm.current_password"
                  :type="showCurrentPassword ? 'text' : 'password'"
                  :placeholder="$t('profile.currentPasswordPlaceholder')"
                />
                <button
                  type="button"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  @click="showCurrentPassword = !showCurrentPassword"
                >
                  <Eye v-if="!showCurrentPassword" class="h-4 w-4" />
                  <EyeOff v-else class="h-4 w-4" />
                </button>
              </div>
            </div>
            <div class="space-y-2">
              <Label for="new_password">{{ $t('profile.newPassword') }}</Label>
              <div class="relative">
                <Input
                  id="new_password"
                  v-model="passwordForm.new_password"
                  :type="showNewPassword ? 'text' : 'password'"
                  :placeholder="$t('profile.newPasswordPlaceholder')"
                />
                <button
                  type="button"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  @click="showNewPassword = !showNewPassword"
                >
                  <Eye v-if="!showNewPassword" class="h-4 w-4" />
                  <EyeOff v-else class="h-4 w-4" />
                </button>
              </div>
              <p class="text-xs text-muted-foreground">{{ $t('profile.passwordMinLength') }}</p>
            </div>
            <div class="space-y-2">
              <Label for="confirm_password">{{ $t('profile.confirmNewPassword') }}</Label>
              <div class="relative">
                <Input
                  id="confirm_password"
                  v-model="passwordForm.confirm_password"
                  :type="showConfirmPassword ? 'text' : 'password'"
                  :placeholder="$t('profile.confirmNewPasswordPlaceholder')"
                />
                <button
                  type="button"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  @click="showConfirmPassword = !showConfirmPassword"
                >
                  <Eye v-if="!showConfirmPassword" class="h-4 w-4" />
                  <EyeOff v-else class="h-4 w-4" />
                </button>
              </div>
            </div>
            <div class="flex justify-end">
              <Button variant="outline" size="sm" @click="changePassword" :disabled="isChangingPassword">
                <Loader2 v-if="isChangingPassword" class="mr-2 h-4 w-4 animate-spin" />
                {{ $t('profile.changePassword') }}
              </Button>
            </div>
          </CardContent>
        </Card>

        <!-- Two-Factor Authentication -->
        <Card>
          <CardHeader>
            <CardTitle>Two-Factor Authentication</CardTitle>
            <CardDescription>
              Add an extra layer of security to your account.
            </CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="flex items-center justify-between">
              <p class="text-sm text-muted-foreground">
                Status: <span class="font-medium text-foreground">{{ authStore.user?.totp_enabled ? 'Enabled' : 'Disabled' }}</span>
              </p>
              <div class="flex items-center gap-2">
                <Button v-if="!authStore.user?.totp_enabled" variant="outline" size="sm" @click="setupTwoFA" :disabled="isSettingUpTwoFA">
                  <Loader2 v-if="isSettingUpTwoFA" class="mr-2 h-4 w-4 animate-spin" />
                  Set up 2FA
                </Button>
                <template v-else>
                  <Button variant="outline" size="sm" @click="resetDialogOpen = true">Reset 2FA</Button>
                  <Button variant="outline" size="sm" @click="disableDialogOpen = true">Disable 2FA</Button>
                </template>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </ScrollArea>
  </div>

  <Dialog v-model:open="setupDialogOpen">
    <DialogContent class="sm:max-w-[480px]">
      <form @submit.prevent="verifyTwoFA" class="space-y-4">
        <DialogHeader>
          <DialogTitle>Set Up Two-Factor Authentication</DialogTitle>
          <DialogDescription>Scan the QR code and enter the verification code.</DialogDescription>
        </DialogHeader>
        <div class="space-y-3">
          <div class="rounded-lg border p-4 flex flex-col items-center gap-2">
            <img :src="twoFASetup.qr_code" alt="TOTP QR Code" class="h-40 w-40" />
            <p class="text-xs text-muted-foreground">Scan with your authenticator app</p>
          </div>
          <div class="space-y-1">
            <Label>Manual entry key</Label>
            <Input :model-value="twoFASetup.secret" readonly />
          </div>
          <div class="space-y-1">
            <Label for="twofa_code">Verification Code</Label>
            <Input id="twofa_code" v-model="twoFASetup.code" type="text" placeholder="123456" autocomplete="one-time-code" />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="setupDialogOpen = false">Cancel</Button>
          <Button variant="outline" type="submit" :disabled="isVerifyingTwoFA">
            <Loader2 v-if="isVerifyingTwoFA" class="mr-2 h-4 w-4 animate-spin" />
            Enable 2FA
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>

  <Dialog v-model:open="resetDialogOpen">
    <DialogContent class="sm:max-w-[480px]">
      <form @submit.prevent="verifyResetTwoFA" class="space-y-4">
        <DialogHeader>
          <DialogTitle>Reset Two-Factor Authentication</DialogTitle>
          <DialogDescription>Verify your password, then scan the new QR code.</DialogDescription>
        </DialogHeader>
        <div class="space-y-3">
          <div class="space-y-2">
            <Label for="reset_password">Current Password</Label>
            <div class="relative">
              <Input id="reset_password" v-model="twoFAReset.current_password" :type="showResetPassword ? 'text' : 'password'" placeholder="Enter current password" />
              <button type="button" class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground" @click="showResetPassword = !showResetPassword">
                <Eye v-if="!showResetPassword" class="h-4 w-4" />
                <EyeOff v-else class="h-4 w-4" />
              </button>
            </div>
            <Button variant="outline" size="sm" @click="resetTwoFA" :disabled="isResettingTwoFA">
              <Loader2 v-if="isResettingTwoFA" class="mr-2 h-4 w-4 animate-spin" />
              Generate New QR Code
            </Button>
          </div>
          <div v-if="twoFAReset.qr_code" class="space-y-2">
            <div class="rounded-lg border p-4 flex flex-col items-center gap-2">
              <img :src="twoFAReset.qr_code" alt="TOTP QR Code" class="h-40 w-40" />
              <p class="text-xs text-muted-foreground">Scan with your authenticator app</p>
            </div>
            <div class="space-y-1">
              <Label>Manual entry key</Label>
              <Input :model-value="twoFAReset.secret" readonly />
            </div>
            <div class="space-y-1">
              <Label for="reset_code">Verification Code</Label>
              <Input id="reset_code" v-model="twoFAReset.code" type="text" placeholder="123456" autocomplete="one-time-code" />
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="resetDialogOpen = false">Cancel</Button>
          <Button variant="outline" type="submit" :disabled="isVerifyingTwoFA || !twoFAReset.qr_code">
            <Loader2 v-if="isVerifyingTwoFA" class="mr-2 h-4 w-4 animate-spin" />
            Verify & Save
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>

  <Dialog v-model:open="disableDialogOpen">
    <DialogContent class="sm:max-w-[420px]">
      <form @submit.prevent="disableTwoFA" class="space-y-4">
        <DialogHeader>
          <DialogTitle>Disable Two-Factor Authentication</DialogTitle>
          <DialogDescription>Enter your password to disable 2FA.</DialogDescription>
        </DialogHeader>
        <div class="space-y-2">
          <Label for="disable_password">Current Password</Label>
          <div class="relative">
            <Input id="disable_password" v-model="twoFADisable.current_password" :type="showDisablePassword ? 'text' : 'password'" placeholder="Enter current password" />
            <button type="button" class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground" @click="showDisablePassword = !showDisablePassword">
              <Eye v-if="!showDisablePassword" class="h-4 w-4" />
              <EyeOff v-else class="h-4 w-4" />
            </button>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="disableDialogOpen = false">Cancel</Button>
          <Button variant="outline" type="submit" :disabled="isDisablingTwoFA">
            <Loader2 v-if="isDisablingTwoFA" class="mr-2 h-4 w-4 animate-spin" />
            Disable 2FA
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
