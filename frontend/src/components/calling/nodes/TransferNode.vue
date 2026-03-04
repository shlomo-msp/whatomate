<script setup lang="ts">
import { computed } from 'vue'
import { Users } from 'lucide-vue-next'
import BaseNode from './BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: Record<string, any>; teamName?: string }>()

const summary = computed(() => props.teamName || props.data?.config?.team_id || 'No team')
</script>

<template>
  <BaseNode :label="data?.label || 'Transfer'" header-class="bg-amber-600" :output-handles="[]" :has-input="!data?.isEntryNode">
    <template #icon><Users class="w-3 h-3" /></template>
    <p class="truncate">{{ summary }}</p>
  </BaseNode>
</template>
