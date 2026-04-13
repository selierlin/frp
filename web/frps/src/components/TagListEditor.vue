<template>
  <div class="tag-list-editor">
    <el-tag
      v-for="tag in modelValue"
      :key="tag"
      closable
      class="tag-item"
      @close="handleClose(tag)"
    >
      {{ tag }}
    </el-tag>

    <template v-if="inputVisible">
      <div class="tag-input-wrapper">
        <el-input
          ref="inputRef"
          v-model="inputValue"
          size="small"
          :placeholder="placeholder"
          :class="{ 'is-error': inputError }"
          @keyup.enter="handleInputConfirm"
          @keyup.esc="handleCancel"
          @blur="handleInputConfirm"
        />
        <div v-if="inputError" class="tag-input-error">{{ inputError }}</div>
      </div>
    </template>
    <el-button v-else size="small" class="add-tag-btn" @click="showInput">
      + Add
    </el-button>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue: string[]
    placeholder?: string
    validator?: (v: string) => boolean
    validatorMessage?: string
  }>(),
  {
    placeholder: '',
    validator: undefined,
    validatorMessage: 'Invalid value',
  },
)

const emit = defineEmits<{
  'update:modelValue': [val: string[]]
}>()

const inputVisible = ref(false)
const inputValue = ref('')
const inputError = ref('')
const inputRef = ref<InstanceType<typeof import('element-plus')['ElInput']> | null>(null)

const showInput = () => {
  inputVisible.value = true
  inputError.value = ''
  nextTick(() => inputRef.value?.focus())
}

const handleClose = (tag: string) => {
  emit('update:modelValue', props.modelValue.filter((t) => t !== tag))
}

const handleCancel = () => {
  inputVisible.value = false
  inputValue.value = ''
  inputError.value = ''
}

const handleInputConfirm = () => {
  const val = inputValue.value.trim()
  if (!val) {
    handleCancel()
    return
  }
  if (props.validator && !props.validator(val)) {
    inputError.value = props.validatorMessage || 'Invalid value'
    return
  }
  if (!props.modelValue.includes(val)) {
    emit('update:modelValue', [...props.modelValue, val])
  }
  inputVisible.value = false
  inputValue.value = ''
  inputError.value = ''
}
</script>

<style scoped>
.tag-list-editor {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.tag-item {
  font-size: 13px;
}

.tag-input-wrapper {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.tag-input-error {
  font-size: 12px;
  color: var(--el-color-danger);
}

.add-tag-btn {
  border-style: dashed;
}
</style>
