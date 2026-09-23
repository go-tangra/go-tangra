<script setup lang="ts">
import { ref } from 'vue'
import { z } from 'zod'
import { UiPage, UiGrid, UiCard, UiForm, UiInput, UiSelect, UiTextarea, UiNumberInput, UiButton, UiRecordForm, UiKeyValueTable, useToast, type SelectOption } from '@/index'
import { useZodForm, nonEmpty, email, money, isoDate, optionalString, tagMap, dateRange } from '@/forms'
import { ApiError } from '@/api'

// Hand-written form on useZodForm: one schema drives validation and the payload.
const Contact = dateRange(z.object({
  name: nonEmpty(60),
  email,
  kind: z.enum(['person', 'org']),
  budget: money,
  valid_from: isoDate,
  valid_to: isoDate,
  notes: optionalString(500),
}), 'valid_from', 'valid_to')
const kinds: SelectOption[] = [{ title: 'Person', value: 'person' }, { title: 'Organisation', value: 'org' }]
const toast = useToast()
const payload = ref<unknown>()
const failNext = ref<'none' | 'conflict' | 'fields'>('none')
const form = useZodForm(Contact, {
  initial: { kind: 'person' },
  onSubmit: async (p) => {
    if (failNext.value === 'conflict') throw new ApiError(409, 'conflict')
    if (failNext.value === 'fields') throw new ApiError(422, 'validation_failed', { fields: { email: 'Already registered.' } })
    payload.value = p
    return p
  },
  onSuccess: () => toast.success('Submitted'),
})

// Schema-driven form: zodToFields derives the layout.
const Quick = z.object({ title: nonEmpty(40), amount: money, category: z.enum(['a', 'b']), due_at: isoDate, description: optionalString(200), tags: tagMap(4), active: z.boolean().optional(), api_token: optionalString(64) })
const quick = ref<unknown>()
</script>

<template>
  <UiPage title="Forms" subtitle="useZodForm + shared schemas; UiRecordForm from zodToFields">
    <UiGrid :cols="1" :lg-cols="2">
      <UiCard title="Hand-written form (useZodForm)">
        <UiForm :form="form">
          <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
            <UiInput v-bind="form.field('name')" label="Name" required />
            <UiInput v-bind="form.field('email')" label="Email" type="email" required />
            <UiSelect v-bind="form.field('kind')" label="Kind" :options="kinds" required />
            <UiNumberInput v-bind="form.field('budget')" label="Budget" :min="0" :step="0.01" />
            <UiInput v-bind="form.field('valid_from')" label="Valid from" type="date" />
            <UiInput v-bind="form.field('valid_to')" label="Valid to" type="date" hint="Must not precede 'valid from'" />
            <div class="md:col-span-2"><UiTextarea v-bind="form.field('notes')" label="Notes" /></div>
          </div>
          <div class="mt-3 flex flex-wrap items-center gap-2">
            <UiButton type="submit" :loading="form.submitting.value">Submit</UiButton>
            <UiButton variant="text" @click="form.reset()">Reset</UiButton>
            <UiSelect id="fail-next" v-model="failNext" label="Server answer" sr-only-label size="sm" :clearable="false" :options="[{ title: 'accept', value: 'none' }, { title: 'refuse: conflict', value: 'conflict' }, { title: 'refuse: field errors', value: 'fields' }]" />
            <span class="text-xs text-base-content/70">dirty: {{ form.dirty.value }} · valid: {{ form.valid.value }}</span>
          </div>
        </UiForm>
        <UiKeyValueTable v-if="payload" class="mt-4" :items="[{ label: 'z.output', value: payload }]" />
      </UiCard>
      <UiCard title="Schema-driven (UiRecordForm)">
        <UiRecordForm :schema="Quick" :submit="async (v) => { quick = v; return v }" :initial="{ category: 'a' }" @saved="toast.success('Saved')">
          <template #default><UiButton type="submit" class="mt-3">Save</UiButton></template>
        </UiRecordForm>
        <UiKeyValueTable v-if="quick" class="mt-4" :items="[{ label: 'payload', value: quick }]" />
      </UiCard>
    </UiGrid>
  </UiPage>
</template>
