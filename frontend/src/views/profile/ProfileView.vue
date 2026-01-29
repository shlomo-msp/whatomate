<script setup lang="ts">
import { ref } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { toast } from 'vue-sonner'
import { User, Eye, EyeOff, Loader2 } from 'lucide-vue-next'
import { usersService } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { PageHeader } from '@/components/shared'
import { getErrorMessage } from '@/lib/api-utils'

const authStore = useAuthStore()
const isChangingPassword = ref(false)
const isSettingUpTwoFA = ref(false)
const isVerifyingTwoFA = ref(false)
const isDisablingTwoFA = ref(false)
const showCurrentPassword = ref(false)
const showNewPassword = ref(false)
const showConfirmPassword = ref(false)
const showTwoFAPassword = ref(false)
const showDisablePassword = ref(false)

const passwordForm = ref({
  current_password: '',
  new_password: '',
  confirm_password: ''
})

const twoFASetup = ref({
  current_password: '',
  secret: '',
  otpauth_url: '',
  qr_code: '',
  code: ''
})

const twoFADisable = ref({
  current_password: '',
  code: ''
})

async function changePassword() {
  // Validate passwords match
  if (passwordForm.value.new_password !== passwordForm.value.confirm_password) {
    toast.error('New passwords do not match')
    return
  }

  // Validate password length
  if (passwordForm.value.new_password.length < 6) {
    toast.error('New password must be at least 6 characters')
    return
  }

  isChangingPassword.value = true
  try {
    await usersService.changePassword({
      current_password: passwordForm.value.current_password,
      new_password: passwordForm.value.new_password
    })
    toast.success('Password changed successfully')
    // Clear the form
    passwordForm.value = {
      current_password: '',
      new_password: '',
      confirm_password: ''
    }
  } catch (error: any) {
    toast.error(getErrorMessage(error, 'Failed to change password'))
  } finally {
    isChangingPassword.value = false
  }
}

async function setupTwoFA() {
  if (!twoFASetup.value.current_password) {
    toast.error('Enter your current password')
    return
  }

  isSettingUpTwoFA.value = true
  try {
    const response = await usersService.setupTwoFA(twoFASetup.value.current_password)
    const data = response.data.data
    twoFASetup.value.secret = data.secret
    twoFASetup.value.otpauth_url = data.otpauth_url
    twoFASetup.value.qr_code = data.qr_code
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
    twoFASetup.value = { current_password: '', secret: '', otpauth_url: '', qr_code: '', code: '' }
    await authStore.refreshUserData()
  } catch (error: any) {
    toast.error(getErrorMessage(error, 'Failed to verify 2FA'))
  } finally {
    isVerifyingTwoFA.value = false
  }
}

async function disableTwoFA() {
  if (!twoFADisable.value.current_password || !twoFADisable.value.code) {
    toast.error('Enter your password and verification code')
    return
  }

  isDisablingTwoFA.value = true
  try {
    await usersService.disableTwoFA(twoFADisable.value.current_password, twoFADisable.value.code)
    toast.success('Two-factor authentication disabled')
    twoFADisable.value = { current_password: '', code: '' }
    await authStore.refreshUserData()
  } catch (error: any) {
    toast.error(getErrorMessage(error, 'Failed to disable 2FA'))
  } finally {
    isDisablingTwoFA.value = false
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <PageHeader
      title="Profile"
      description="Manage your account settings"
      :icon="User"
      icon-gradient="bg-gradient-to-br from-gray-500 to-gray-600 shadow-gray-500/20"
    />

    <!-- Content -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-6 max-w-2xl mx-auto">
        <!-- User Info -->
        <Card>
          <CardHeader>
            <CardTitle>Account Information</CardTitle>
            <CardDescription>Your account details</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="grid grid-cols-2 gap-4">
              <div>
                <Label class="text-muted-foreground">Name</Label>
                <p class="font-medium">{{ authStore.user?.full_name }}</p>
              </div>
              <div>
                <Label class="text-muted-foreground">Email</Label>
                <p class="font-medium">{{ authStore.user?.email }}</p>
              </div>
              <div>
                <Label class="text-muted-foreground">Role</Label>
                <p class="font-medium capitalize">{{ authStore.user?.role?.name }}</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- Change Password -->
        <Card>
          <CardHeader>
            <CardTitle>Change Password</CardTitle>
            <CardDescription>Update your account password</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="space-y-2">
              <Label for="current_password">Current Password</Label>
              <div class="relative">
                <Input
                  id="current_password"
                  v-model="passwordForm.current_password"
                  :type="showCurrentPassword ? 'text' : 'password'"
                  placeholder="Enter current password"
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
              <Label for="new_password">New Password</Label>
              <div class="relative">
                <Input
                  id="new_password"
                  v-model="passwordForm.new_password"
                  :type="showNewPassword ? 'text' : 'password'"
                  placeholder="Enter new password"
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
              <p class="text-xs text-muted-foreground">Must be at least 6 characters</p>
            </div>
            <div class="space-y-2">
              <Label for="confirm_password">Confirm New Password</Label>
              <div class="relative">
                <Input
                  id="confirm_password"
                  v-model="passwordForm.confirm_password"
                  :type="showConfirmPassword ? 'text' : 'password'"
                  placeholder="Confirm new password"
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
                Change Password
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
            </div>

            <div v-if="!authStore.user?.totp_enabled" class="space-y-4">
              <div class="space-y-2">
                <Label for="twofa_password">Current Password</Label>
                <div class="relative">
                  <Input
                    id="twofa_password"
                    v-model="twoFASetup.current_password"
                    :type="showTwoFAPassword ? 'text' : 'password'"
                    placeholder="Enter current password"
                  />
                  <button
                    type="button"
                    class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                    @click="showTwoFAPassword = !showTwoFAPassword"
                  >
                    <Eye v-if="!showTwoFAPassword" class="h-4 w-4" />
                    <EyeOff v-else class="h-4 w-4" />
                  </button>
                </div>
              </div>

              <Button variant="outline" size="sm" @click="setupTwoFA" :disabled="isSettingUpTwoFA">
                <Loader2 v-if="isSettingUpTwoFA" class="mr-2 h-4 w-4 animate-spin" />
                Start 2FA Setup
              </Button>

              <div v-if="twoFASetup.qr_code" class="space-y-3">
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
                  <Input
                    id="twofa_code"
                    v-model="twoFASetup.code"
                    type="text"
                    placeholder="123456"
                    autocomplete="one-time-code"
                  />
                </div>
                <div class="flex justify-end">
                  <Button variant="outline" size="sm" @click="verifyTwoFA" :disabled="isVerifyingTwoFA">
                    <Loader2 v-if="isVerifyingTwoFA" class="mr-2 h-4 w-4 animate-spin" />
                    Enable 2FA
                  </Button>
                </div>
              </div>
            </div>

            <div v-else class="space-y-3">
              <div class="space-y-2">
                <Label for="disable_password">Current Password</Label>
                <div class="relative">
                  <Input
                    id="disable_password"
                    v-model="twoFADisable.current_password"
                    :type="showDisablePassword ? 'text' : 'password'"
                    placeholder="Enter current password"
                  />
                  <button
                    type="button"
                    class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                    @click="showDisablePassword = !showDisablePassword"
                  >
                    <Eye v-if="!showDisablePassword" class="h-4 w-4" />
                    <EyeOff v-else class="h-4 w-4" />
                  </button>
                </div>
              </div>
              <div class="space-y-2">
                <Label for="disable_code">Verification Code</Label>
                <Input
                  id="disable_code"
                  v-model="twoFADisable.code"
                  type="text"
                  placeholder="123456"
                  autocomplete="one-time-code"
                />
              </div>
              <div class="flex justify-end">
                <Button variant="outline" size="sm" @click="disableTwoFA" :disabled="isDisablingTwoFA">
                  <Loader2 v-if="isDisablingTwoFA" class="mr-2 h-4 w-4 animate-spin" />
                  Disable 2FA
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </ScrollArea>
  </div>
</template>
