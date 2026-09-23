<script setup lang="ts">
import { ref } from 'vue'
import { UiPage, UiSection, UiGrid, UiField, UiInput, UiTextarea, UiSelect, UiCombobox, UiCheckbox, UiSwitch, UiDateInput, UiNumberInput, UiFilePicker, UiSecretField, UiTagEditor, type SelectOption } from '@/index'

const text = ref('')
const notes = ref('')
const sel = ref<string | undefined>()
const combo = ref<string | undefined>()
const check = ref(false)
const sw = ref(true)
const date = ref<string | undefined>()
const num = ref<number | undefined>(3)
const file = ref<File | null>(null)
const secret = ref('')
const tags = ref<Record<string, string>>({ env: 'lab' })
const options: SelectOption[] = [{ title: 'Alpha', value: 'a' }, { title: 'Beta', value: 'b' }, { title: 'Gamma', value: 'c' }]
</script>

<template>
  <UiPage title="Fields" subtitle="Every input with label, hint, error and disabled variants">
    <UiSection title="Text-like">
      <UiGrid :cols="1" :lg-cols="3">
        <UiInput id="f-text" v-model="text" label="Text" hint="A hint" placeholder="Type…" required />
        <UiInput id="f-err" v-model="text" label="With error" error="This field is required." />
        <UiInput id="f-dis" label="Disabled" disabled model-value="fixed" />
        <UiInput id="f-email" label="Email" type="email" size="sm" />
        <UiTextarea id="f-notes" v-model="notes" label="Notes" :rows="2" placeholder="Free text" />
        <UiSecretField id="f-secret" v-model="secret" label="Secret" hint="Masked by default" />
        <UiSecretField id="f-secret2" label="Secret (no reveal)" :revealable="false" autocomplete="new-password" />
        <UiNumberInput id="f-num" v-model="num" label="Number" :min="0" :max="10" :step="1" />
        <UiDateInput id="f-date" v-model="date" label="Date" hint="ISO date" />
      </UiGrid>
    </UiSection>
    <UiSection title="Choice">
      <UiGrid :cols="1" :lg-cols="3">
        <UiSelect id="f-sel" v-model="sel" label="Select" :options="options" hint="Clearable" />
        <UiSelect id="f-sel2" label="Select (error, sm)" :options="options" error="Pick one." size="sm" :clearable="false" />
        <UiCombobox id="f-combo" v-model="combo" label="Combobox" :options="options" placeholder="Search…" />
        <UiCheckbox id="f-check" v-model="check" label="Checkbox" hint="Optional" />
        <UiCheckbox id="f-check2" label="Checkbox with error" error="Must accept." />
        <UiSwitch id="f-sw" v-model="sw" label="Switch" />
      </UiGrid>
    </UiSection>
    <UiSection title="Files and tags">
      <UiGrid :cols="1" :lg-cols="2">
        <UiFilePicker id="f-file" v-model="file" label="File" accept=".pdf,image/*" :max-bytes="1048576" hint="≤ 1 MiB" />
        <UiTagEditor id="f-tags" v-model="tags" label="Tags" :max="5" hint="key=value pairs" />
        <UiField id="f-custom" label="Custom field" hint="UiField wraps any control">
          <template #default="{ describedBy, invalid }"><input id="f-custom" class="input" :aria-describedby="describedBy" :aria-invalid="invalid || undefined"></template>
        </UiField>
      </UiGrid>
    </UiSection>
  </UiPage>
</template>
