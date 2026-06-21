<script setup>
defineProps({
  color: {
    type: String,
    default: 'grey'
  }
})
</script>

<template>
  <span :class="['status-dot', `status-dot--${color}`]" />
</template>
