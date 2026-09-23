<script setup lang="ts">
// Create/edit dialog driven by a Zod schema (+ optional field definitions).
// Emits `saved` with the submit result and closes; server refusals show inline.
import { shallowRef } from 'vue'
import type { z } from 'zod'
import type { ZodForm } from '@/forms/useZodForm'
import type { FieldDef } from '@/forms/zodToFields'
import UiDialog from './UiDialog.vue'
import UiRecordForm from './UiRecordForm.vue'
import UiButton from './UiButton.vue'

const props = withDefaults(defineProps<{
  modelValue: boolean
  title: string
  schema: z.ZodObject<z.ZodRawShape>
  fields?: FieldDef[] | undefined
  initial?: Record<string, unknown> | undefined
  submit: (values: Record<string, unknown>) => Promise<unknown>
  size?: 'sm' | 'md' | 'lg' | 'xl' | undefined
  saveLabel?: string | undefined
}>(), { size: 'md', saveLabel: 'Save' })
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void; (e: 'saved', v: unknown): void }>()
const form = shallowRef<ZodForm<z.ZodType> | null>(null)
function close() {
  emit('update:modelValue', false)
}
function onSaved(v: unknown) {
  emit('saved', v)
  close()
}
</script>

<template>
  <UiDialog :model-value="modelValue" :title="title" :size="size" @update:model-value="close">
    <UiRecordForm v-if="modelValue" :schema="schema" :fields="fields" :initial="initial" :submit="props.submit" @saved="onSaved" @ready="form = $event">
      <template v-for="(_, name) in $slots" #[name]="scope"><slot :name="name" v-bind="scope ?? {}" /></template>
    </UiRecordForm>
    <template #actions>
      <UiButton variant="text" color="neutral" @click="close">Cancel</UiButton>
      <UiButton :loading="form?.submitting.value ?? false" @click="form?.submit()">{{ saveLabel }}</UiButton>
    </template>
  </UiDialog>
</template>
