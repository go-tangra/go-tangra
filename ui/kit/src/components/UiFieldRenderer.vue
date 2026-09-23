<script setup lang="ts">
// Renders one FieldDef bound to a ZodForm (used by UiRecordDialog/UiRecordDrawer).
import type { FieldDef } from '@/forms/zodToFields'
import type { ZodForm } from '@/forms/useZodForm'
import type { z } from 'zod'
import UiInput from './UiInput.vue'
import UiTextarea from './UiTextarea.vue'
import UiSelect from './UiSelect.vue'
import UiCombobox from './UiCombobox.vue'
import UiCheckbox from './UiCheckbox.vue'
import UiDateInput from './UiDateInput.vue'
import UiNumberInput from './UiNumberInput.vue'
import UiSecretField from './UiSecretField.vue'
import UiTagEditor from './UiTagEditor.vue'

const props = defineProps<{ field: FieldDef; form: ZodForm<z.ZodType> }>()
const val = () => props.form.values[props.field.key]
// eslint-disable-next-line vue/no-mutating-props -- the form object is a shared reactive model, not a prop value
const set = (v: unknown) => { props.form.values[props.field.key] = v }
const blur = () => props.form.blur(props.field.key)
</script>

<template>
  <UiSelect v-if="field.type === 'select' && (field.options?.length ?? 0) <= 12" :id="field.key" :label="field.label" :options="field.options ?? []" :hint="field.hint" :error="form.errors.value[field.key]" :required="field.required" :model-value="val()" @update:model-value="set" @blur="blur" />
  <UiCombobox v-else-if="field.type === 'select'" :id="field.key" :label="field.label" :options="field.options ?? []" :hint="field.hint" :error="form.errors.value[field.key]" :required="field.required" :model-value="val()" @update:model-value="set" @blur="blur" />
  <UiTextarea v-else-if="field.type === 'textarea'" :id="field.key" :label="field.label" :hint="field.hint" :error="form.errors.value[field.key]" :required="field.required" :placeholder="field.placeholder" :model-value="val()" @update:model-value="set" @blur="blur" />
  <UiCheckbox v-else-if="field.type === 'checkbox'" :id="field.key" :label="field.label" :hint="field.hint" :error="form.errors.value[field.key]" :model-value="val()" @update:model-value="set" @blur="blur" />
  <UiDateInput v-else-if="field.type === 'date'" :id="field.key" :label="field.label" :hint="field.hint" :error="form.errors.value[field.key]" :required="field.required" :model-value="val()" @update:model-value="set" @blur="blur" />
  <UiNumberInput v-else-if="field.type === 'number'" :id="field.key" :label="field.label" :hint="field.hint" :error="form.errors.value[field.key]" :required="field.required" :model-value="val()" @update:model-value="set" @blur="blur" />
  <UiSecretField v-else-if="field.type === 'secret'" :id="field.key" :label="field.label" :hint="field.hint" :error="form.errors.value[field.key]" :required="field.required" :model-value="val()" @update:model-value="set" @blur="blur" />
  <UiTagEditor v-else-if="field.type === 'tags'" :id="field.key" :label="field.label" :hint="field.hint" :error="form.errors.value[field.key]" :model-value="(val() as Record<string, string> | undefined) ?? {}" @update:model-value="set" @blur="blur" />
  <UiInput v-else :id="field.key" :label="field.label" :hint="field.hint" :error="form.errors.value[field.key]" :required="field.required" :placeholder="field.placeholder" :model-value="val()" @update:model-value="set" @blur="blur" />
</template>
