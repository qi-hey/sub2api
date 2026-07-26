<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.grokSSOReauth.title')"
    width="wide"
    close-on-click-outside
    @close="handleClose"
  >
    <div class="space-y-4">
      <div class="flex flex-wrap items-center gap-2">
        <input
          ref="fileInput"
          type="file"
          accept=".txt,.csv,text/plain,text/csv"
          class="hidden"
          @change="handleFileChange"
        />
        <button type="button" class="btn btn-secondary" :disabled="busy" @click="fileInput?.click()">
          <Icon name="upload" size="sm" class="mr-2" />
          {{ t('admin.accounts.grokSSOReauth.upload') }}
        </button>
        <span v-if="selectedFileName" class="max-w-full truncate text-sm text-gray-500 dark:text-dark-400">
          {{ selectedFileName }}
        </span>
      </div>

      <div>
        <label class="input-label" for="grok-sso-reauth-input">
          {{ t('admin.accounts.grokSSOReauth.tokens') }}
        </label>
        <textarea
          id="grok-sso-reauth-input"
          v-model="tokenInput"
          class="input min-h-40 w-full resize-y font-mono text-xs"
          :placeholder="t('admin.accounts.grokSSOReauth.placeholder')"
          :disabled="busy"
          @input="resetPreview"
        />
        <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
          {{ t('admin.accounts.grokSSOReauth.tokenCount', { count: tokens.length }) }}
        </div>
        <div
          v-if="tokens.length"
          class="mt-2 rounded border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-200"
        >
          <span v-if="!previewComplete">
            {{ t('admin.accounts.grokSSOReauth.previewRequired') }}
          </span>
          <span v-else>
            {{
              t('admin.accounts.grokSSOReauth.previewStats', {
                executable: applicableSelection.tokens.length,
                skipped: applicableSelection.skipped
              })
            }}
          </span>
        </div>
      </div>

      <div
        v-if="previewItems.length"
        class="max-h-80 overflow-auto rounded-lg border border-gray-200 dark:border-dark-600"
      >
        <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="sticky top-0 bg-gray-50 dark:bg-dark-700">
            <tr>
              <th class="px-3 py-2 text-left text-xs font-medium text-gray-500">#</th>
              <th class="px-3 py-2 text-left text-xs font-medium text-gray-500">
                {{ t('admin.accounts.grokSSOReauth.identity') }}
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium text-gray-500">
                {{ t('admin.accounts.grokSSOReauth.target') }}
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium text-gray-500">
                {{ t('admin.accounts.grokSSOReauth.action') }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-800">
            <tr v-for="item in previewItems" :key="item.index">
              <td class="px-3 py-2 text-gray-500">{{ item.index }}</td>
              <td class="max-w-64 px-3 py-2">
                <div class="truncate text-gray-900 dark:text-white" :title="item.email || item.sub">
                  {{ item.email || item.sub || '-' }}
                </div>
              </td>
              <td class="max-w-64 px-3 py-2">
                <div class="truncate text-gray-700 dark:text-dark-200" :title="item.account_name">
                  {{ item.account_name || '-' }}
                </div>
                <div v-if="item.account_id" class="text-xs text-gray-400">ID {{ item.account_id }}</div>
              </td>
              <td class="px-3 py-2">
                <span :class="previewBadgeClass(item)">
                  {{ previewActionLabel(item) }}
                </span>
                <div v-if="item.error" class="mt-1 max-w-sm text-xs text-red-600 dark:text-red-400">
                  {{ item.error }}
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div
        v-if="applySummary"
        class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-200"
      >
        {{ applySummary }}
      </div>
    </div>

    <template #footer>
      <div class="flex w-full flex-wrap justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="busy" @click="handleClose">
          {{ t('common.close') }}
        </button>
        <button type="button" class="btn btn-secondary" :disabled="busy || !tokens.length" @click="preview">
          <Icon name="eye" size="sm" class="mr-2" />
          {{ previewing ? t('admin.accounts.grokSSOReauth.previewing') : t('admin.accounts.grokSSOReauth.preview') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="busy || !canApply"
          :title="!canApply ? applyDisabledReason : undefined"
          @click="apply"
        >
          <Icon name="refresh" size="sm" class="mr-2" />
          {{ applying ? t('admin.accounts.grokSSOReauth.applying') : t('admin.accounts.grokSSOReauth.apply') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { GrokSSOReauthPreviewItem } from '@/api/admin/grok'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { selectApplicableGrokSSOTokens } from './grokSSOReauth'

interface Props {
  show: boolean
}

interface Emits {
  (e: 'close'): void
  (e: 'updated'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const { t } = useI18n()
const appStore = useAppStore()

const fileInput = ref<HTMLInputElement | null>(null)
const selectedFileName = ref('')
const tokenInput = ref('')
const previewItems = ref<GrokSSOReauthPreviewItem[]>([])
const previewComplete = ref(false)
const previewing = ref(false)
const applying = ref(false)
const applySummary = ref('')
const busy = computed(() => previewing.value || applying.value)
const tokens = computed(() => {
  const normalized = tokenInput.value.replace(/\r/g, '\n').replace(/,/g, '\n')
  return [...new Set(normalized.split('\n').map(item => item.trim()).filter(Boolean))]
})
const applicableSelection = computed(() =>
  selectApplicableGrokSSOTokens(tokens.value, previewItems.value)
)
const canApply = computed(() => previewComplete.value && applicableSelection.value.tokens.length > 0)
const applyDisabledReason = computed(() =>
  previewComplete.value
    ? t('admin.accounts.grokSSOReauth.noApplicable')
    : t('admin.accounts.grokSSOReauth.previewRequired')
)

watch(
  () => props.show,
  (show) => {
    if (!show) return
    selectedFileName.value = ''
    tokenInput.value = ''
    previewItems.value = []
    previewComplete.value = false
    applySummary.value = ''
  }
)

const resetPreview = () => {
  previewItems.value = []
  previewComplete.value = false
  applySummary.value = ''
}

const handleFileChange = async (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  selectedFileName.value = file.name
  tokenInput.value = await file.text()
  resetPreview()
}

const preview = async () => {
  if (!tokens.value.length) return
  previewing.value = true
  applySummary.value = ''
  try {
    const result = await adminAPI.grok.reauthFromSSO({
      sso_tokens: tokens.value,
      preview: true,
      confirmed: false,
      create_if_missing: false
    })
    previewItems.value = result.preview || []
    previewComplete.value = true
    if (!applicableSelection.value.tokens.length) {
      appStore.showWarning(t('admin.accounts.grokSSOReauth.noApplicable'))
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.grokSSOReauth.previewFailed')))
  } finally {
    previewing.value = false
  }
}

const apply = async () => {
  if (!canApply.value) return
  const applicableTokens = [...applicableSelection.value.tokens]
  const skipped = applicableSelection.value.skipped
  applying.value = true
  try {
    const result = await adminAPI.grok.reauthFromSSO({
      sso_tokens: applicableTokens,
      preview: false,
      confirmed: true,
      create_if_missing: false
    })
    const updated = result.updated?.length || 0
    const failed = result.failed?.length || 0
    applySummary.value = t('admin.accounts.grokSSOReauth.summary', { updated, failed, skipped })
    if (failed > 0 || skipped > 0) {
      appStore.showWarning(applySummary.value)
    } else {
      appStore.showSuccess(applySummary.value)
    }
    if (updated > 0) emit('updated')
    previewItems.value = []
    previewComplete.value = false
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.grokSSOReauth.applyFailed')))
  } finally {
    applying.value = false
  }
}

const handleClose = () => {
  if (!busy.value) emit('close')
}

const previewActionLabel = (item: GrokSSOReauthPreviewItem) => {
  const key = ['update', 'create', 'unmatched', 'conflict'].includes(item.action) ? item.action : 'conflict'
  return t(`admin.accounts.grokSSOReauth.actions.${key}`)
}

const previewBadgeClass = (item: GrokSSOReauthPreviewItem) => [
  'inline-flex rounded px-2 py-0.5 text-xs font-medium',
  item.action === 'update' && !item.error
    ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
    : 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
]
</script>
