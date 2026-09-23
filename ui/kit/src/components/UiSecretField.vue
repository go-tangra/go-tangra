<script setup lang="ts">
// Password / token / one-time-code field: masked by default with a reveal
// toggle, autocomplete OFF, never persisted anywhere by the kit.
import { ref } from 'vue'
import UiField from './UiField.vue'
import UiIcon from './UiIcon.vue'
const props = withDefaults(defineProps<{ modelValue?: unknown | undefined; id: string; label: string; hint?: string | undefined; error?: string | undefined; required?: boolean | undefined; disabled?: boolean | undefined; readonly?: boolean | undefined; revealable?: boolean | undefined; autocomplete?: 'off' | 'new-password' | 'current-password' | 'one-time-code' | undefined }>(), { revealable: true, autocomplete: 'off' })
const emit = defineEmits<{ (e: 'update:modelValue', v: unknown): void; (e: 'blur'): void; (e: 'enter'): void }>()
const shown = ref(false)
</script>

<template>
  <UiField :id="id" :label="label" :hint="hint" :error="error" :required="required">
    <template #default="{ describedBy, invalid }">
      <div class="join w-full">
        <input
          :id="id"
          :data-field="id"
          class="input join-item w-full"
          :class="invalid ? 'is-invalid' : ''"
          :type="shown ? 'text' : 'password'"
          :value="props.modelValue ?? ''"
          :disabled="disabled"
          :readonly="readonly"
          :required="required"
          :autocomplete="autocomplete"
          autocapitalize="off"
          autocorrect="off"
          spellcheck="false"
          data-lpignore="true"
          data-1p-ignore="true"
          :aria-invalid="invalid || undefined"
          :aria-describedby="describedBy"
          @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
          @blur="emit('blur')"
          @keyup.enter="emit('enter')"
        >
        <button v-if="revealable" type="button" class="btn btn-outline join-item" :aria-label="shown ? 'Hide value' : 'Show value'" :aria-pressed="shown" @click="shown = !shown"><UiIcon :name="shown ? 'mdi-eye-off-outline' : 'mdi-eye-outline'" size="sm" /></button>
      </div>
    </template>
  </UiField>
</template>
