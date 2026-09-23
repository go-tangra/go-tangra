<script setup lang="ts">
// The single confirm host; mounted once by the shell/console. Driven by useConfirm().
import { computed } from 'vue'
import { useConfirm } from '@/composables/useConfirm'
import UiDialog from './UiDialog.vue'
import UiButton from './UiButton.vue'

const { state, answer } = useConfirm()
const open = computed(() => state.pending !== null)
</script>

<template>
  <UiDialog :model-value="open" :title="state.pending?.title ?? ''" size="sm" @update:model-value="answer(false)">
    <p class="text-sm">{{ state.pending?.text }}</p>
    <template #actions>
      <UiButton variant="text" color="neutral" @click="answer(false)">{{ state.pending?.cancelLabel }}</UiButton>
      <UiButton :color="state.pending?.danger ? 'error' : 'primary'" autofocus @click="answer(true)">{{ state.pending?.confirmLabel }}</UiButton>
    </template>
  </UiDialog>
</template>
