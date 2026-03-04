<script setup lang="ts">
import { computed } from 'vue'
import { ExternalLink } from 'lucide-vue-next'
import BaseNode from './BaseNode.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ data: Record<string, any>; flowName?: string }>()

const summary = computed(() => props.flowName || props.data?.config?.flow_id || 'No flow')
</script>

<template>
  <BaseNode :label="data?.label || 'Goto Flow'" header-class="bg-teal-600" :output-handles="[]" :has-input="!data?.isEntryNode">
    <template #icon><ExternalLink class="w-3 h-3" /></template>
    <p class="truncate">{{ summary }}</p>
  </BaseNode>
</template>
