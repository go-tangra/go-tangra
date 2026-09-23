<script setup lang="ts">
// Side-sheet variant of UiRecordDialog (detail panels that edit in place).
import { shallowRef } from 'vue'
import type { z } from 'zod'
import type { ZodForm } from '@/forms/useZodForm'
import type { FieldDef } from '@/forms/zodToFields'
import UiDrawer from './UiDrawer.vue'
import UiRecordForm from './UiRecordForm.vue'
import UiButton from './UiButton.vue'

const props = withDefaults(defineProps<{
  modelValue: boolean
  title: string
  schema: z.ZodObject<z.ZodRawShape>
  fields?: FieldDef[] | undefined
  initial?: Record<string, unknown> | undefined
  submit: (values: Record<string, unknown>) => Promise<unknown>
  size?: 'md' | 'lg' | 'xl' | undefined
  saveLabel?: string | undefined
  readonly?: boolean | undefined
  /** Close after a successful save (create/edit forms); detail panels stay open. */
  closeOnSave?: boolean | undefined
}>(), { size: 'md', saveLabel: 'Save' })
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void; (e: 'saved', v: unknown): void }>()
const form = shallowRef<ZodForm<z.ZodType> | null>(null)
const close = () => emit('update:modelValue', false)
function onSaved(v: unknown) {
  emit('saved', v)
  if (props.closeOnSave) close()
}
</script>

<template>
  <UiDrawer :model-value="modelValue" :title="title" :size="size" @update:model-value="close">
    <template #header-actions><slot name="header-actions" /></template>
    <slot name="before" />
    <UiRecordForm v-if="modelValue" :schema="schema" :fields="fields" :initial="initial" :submit="props.submit" @saved="onSaved" @ready="form = $event">
      <template v-for="(_, name) in $slots" #[name]="scope"><slot :name="name" v-bind="scope ?? {}" /></template>
    </UiRecordForm>
    <slot name="after" />
    <template v-if="!readonly" #actions>
      <UiButton variant="text" color="neutral" @click="close">Close</UiButton>
      <UiButton :loading="form?.submitting.value ?? false" @click="form?.submit()">{{ saveLabel }}</UiButton>
    </template>
  </UiDrawer>
</template>
