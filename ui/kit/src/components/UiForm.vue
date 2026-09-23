<script setup lang="ts">
// <form> wrapper: submit → the zod form's submit(); shows the server banner;
// `novalidate` so the schema (not the browser) produces the messages.
import type { ZodForm } from '@/forms/useZodForm'
import type { z } from 'zod'
import UiAlert from './UiAlert.vue'
const props = defineProps<{ form: ZodForm<z.ZodType> }>()
async function onSubmit() {
  await props.form.submit()
}
</script>

<template>
  <form novalidate class="flex flex-col gap-4" @submit.prevent="onSubmit">
    <UiAlert v-if="form.serverError.value" kind="error" dismissible @dismiss="form.setServerError(undefined)">{{ form.serverError.value }}</UiAlert>
    <slot />
  </form>
</template>
