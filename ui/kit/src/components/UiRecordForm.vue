<script setup lang="ts">
// Schema-driven form body: builds a ZodForm from `schema`, lays fields out on a
// responsive grid and exposes the form to the parent via v-model:form-ref.
// Used by UiRecordDialog and UiRecordDrawer; can be embedded on a page too.
import { computed, toRaw, watch } from 'vue'
import type { z } from 'zod'
import { useZodForm, type ZodForm } from '@/forms/useZodForm'
import { zodToFields, type FieldDef } from '@/forms/zodToFields'
import UiForm from './UiForm.vue'
import UiFieldRenderer from './UiFieldRenderer.vue'

const props = defineProps<{
  schema: z.ZodObject<z.ZodRawShape>
  fields?: FieldDef[] | undefined
  initial?: Record<string, unknown> | undefined
  submit: (values: Record<string, unknown>) => Promise<unknown>
}>()
const emit = defineEmits<{ (e: 'saved', v: unknown): void; (e: 'ready', form: ZodForm<z.ZodType>): void }>()

// toRaw: a schema handed through reactive state must not be proxied (Zod internals).
const schema = toRaw(props.schema)
const form = useZodForm(schema as unknown as z.ZodType, {
  initial: (props.initial ?? {}) as Record<string, unknown>,
  onSubmit: (payload) => props.submit(payload as Record<string, unknown>),
  onSuccess: (r) => emit('saved', r),
})
emit('ready', form)
watch(() => props.initial, (v) => form.reset((v ?? {}) as Record<string, unknown>))
const defs = computed<FieldDef[]>(() => props.fields ?? zodToFields(toRaw(props.schema)))
const span = (f: FieldDef) => (f.cols === 12 ? 'md:col-span-12' : f.cols === 4 ? 'md:col-span-4' : f.cols === 3 ? 'md:col-span-3' : f.cols === 8 ? 'md:col-span-8' : 'md:col-span-6')
defineExpose({ form })
</script>

<template>
  <UiForm :form="form">
    <div class="grid grid-cols-1 gap-4 md:grid-cols-12">
      <div v-for="f in defs" :key="f.key" :class="span(f)">
        <slot :name="'field-' + f.key" :field="f" :form="form">
          <UiFieldRenderer :field="f" :form="form" />
        </slot>
      </div>
    </div>
    <slot :form="form" />
  </UiForm>
</template>
