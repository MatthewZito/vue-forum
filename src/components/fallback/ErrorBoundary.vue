<script setup>
import {
  ref,
  onErrorCaptured,
  defineProps,
  inject
} from 'vue';

import { isDev } from '@/utils';
import { ERROR_CAT } from '@/services/api/models';

/* Est */
const { event } = inject('$api');

/* Data */
const hasError = ref(false);

const props = defineProps({
  fallback: {
    type: Object
  },
  propagates: {
    type: Boolean,
    default: false
  }
});

onErrorCaptured((err, vm, info) => {
  hasError.value = true;
  // eslint-disable-next-line no-console
  if (isDev) console.warn({ err }, { info });

  event.logError({
    category: ERROR_CAT.BOUND,
    info: `${info} ${err}`
  });

  return !!props.propagates;
});

</script>

<template lang="pug">
div
  div(v-if="hasError")
    component(:is="fallback")
  div(v-else)
    slot(:v-bind="{ props }")
</template>
