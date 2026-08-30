import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const {
  createAccountMock,
  probeUpstreamBillingMock,
  importCodexSessionMock,
  createOpenAICodexPATMock,
  authStoreState,
} = vi.hoisted(() => ({
  createAccountMock: vi.fn(),
  probeUpstreamBillingMock: vi.fn(),
  importCodexSessionMock: vi.fn(),
  createOpenAICodexPATMock: vi.fn(),
  authStoreState: { isSimpleMode: true },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn(),
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStoreState,
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      create: createAccountMock,
      probeUpstreamBilling: probeUpstreamBillingMock,
      checkMixedChannelRisk: vi.fn().mockResolvedValue({ has_risk: false }),
      importCodexSession: importCodexSessionMock,
      createOpenAICodexPAT: createOpenAICodexPATMock,
    },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] }),
      getSettings: vi.fn().mockResolvedValue({}),
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([]),
    },
  },
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn().mockResolvedValue([]),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

import CreateAccountModal from '../CreateAccountModal.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

const OAuthAuthorizationFlowStub = defineComponent({
  name: 'OAuthAuthorizationFlow',
  props: {
    showManualOption: Boolean,
    showCodexSessionImportOption: Boolean,
    showAgentIdentityOption: Boolean,
    showCodexPatOption: Boolean,
    initialInputMethod: String,
  },
  data: () => ({ inputMethod: 'manual' }),
  emits: ['import-codex-session', 'import-codex-pat'],
  template: `
    <div>
      <button data-testid="import-codex-session" @click="$emit('import-codex-session', 'session-json')">session</button>
      <button data-testid="import-codex-pat" @click="$emit('import-codex-pat', 'pat-token')">pat</button>
    </div>
  `,
})

const SelectStub = defineComponent({
  name: 'TestSelect',
  inheritAttrs: false,
  props: {
    modelValue: { type: String, default: '' },
    options: { type: Array, default: () => [] },
  },
  emits: ['update:modelValue'],
  template: '<div v-bind="$attrs" />',
})

const GroupSelectorStub = defineComponent({
  name: 'GroupSelector',
  props: {
    modelValue: {
      type: Array,
      default: () => [],
    },
  },
  emits: ['update:modelValue'],
  template: `
    <button
      type="button"
      data-testid="select-pricing-groups"
      @click="$emit('update:modelValue', [1, 2])"
    >
      groups
    </button>
  `,
})

const ModelWhitelistSelectorStub = defineComponent({
  name: 'ModelWhitelistSelector',
  props: {
    modelValue: {
      type: Array,
      default: () => [],
    },
    platform: String,
    syncCredentials: Object,
  },
  emits: ['update:modelValue'],
  template: '<div data-testid="model-whitelist-selector" />',
})

function mountModal(
  options: {
    groups?: any[]
    simpleMode?: boolean
    stubGroupSelector?: boolean
  } = {}
) {
  authStoreState.isSimpleMode = options.simpleMode ?? true
  return mount(CreateAccountModal, {
    props: { show: true, proxies: [], groups: options.groups ?? [] },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        OAuthAuthorizationFlow: OAuthAuthorizationFlowStub,
        ConfirmDialog: true,
        Select: SelectStub,
        Icon: true,
        PlatformIcon: true,
        ProxySelector: true,
        ProxyAdBanner: true,
        GroupSelector: options.stubGroupSelector ? GroupSelectorStub : false,
        ModelWhitelistSelector: ModelWhitelistSelectorStub,
        QuotaLimitCard: true,
      },
    },
  })
}

const selectableGroups = [
  { id: 1, name: 'Claude', platform: 'anthropic', rate_multiplier: 1, account_count: 1 },
  { id: 2, name: 'Codex', platform: 'openai', rate_multiplier: 1, account_count: 1 },
  { id: 3, name: 'CC Switch', platform: 'openai', rate_multiplier: 1, account_count: 1 },
  { id: 12, name: 'Grok', platform: 'grok', rate_multiplier: 1, account_count: 1 },
]

describe('CreateAccountModal default group selection', () => {
  beforeEach(() => {
    authStoreState.isSimpleMode = true
    createAccountMock.mockReset().mockResolvedValue({ id: 43, platform: 'grok', type: 'apikey' })
  })

  it('selects every compatible group for the current platform by default', async () => {
    const wrapper = mountModal({ groups: selectableGroups, simpleMode: false })
    await flushPromises()

    expect(wrapper.getComponent({ name: 'GroupSelector' }).props('modelValue')).toEqual([1])

    await selectButtonByText(wrapper, 'OpenAI')

    expect(wrapper.getComponent({ name: 'GroupSelector' }).props('modelValue')).toEqual([2, 3])
  })

  it('initializes defaults when groups arrive after the dialog opens', async () => {
    const wrapper = mountModal({ groups: [], simpleMode: false })

    await wrapper.setProps({ groups: selectableGroups })

    expect(wrapper.getComponent({ name: 'GroupSelector' }).props('modelValue')).toEqual([1])
  })

  it('preserves a manual deselection when the group list refreshes', async () => {
    const wrapper = mountModal({ groups: selectableGroups, simpleMode: false })
    await flushPromises()
    await wrapper.get('input[type="checkbox"][value="1"]').setValue(false)

    await wrapper.setProps({
      groups: [
        ...selectableGroups,
        { id: 4, name: 'Claude 2', platform: 'anthropic', rate_multiplier: 1, account_count: 0 },
      ],
    })

    expect(wrapper.getComponent({ name: 'GroupSelector' }).props('modelValue')).toEqual([])
  })

  it('selects Grok groups and restores the Grok mappings on platform return', async () => {
    const wrapper = mountModal({ groups: selectableGroups, simpleMode: false })
    await flushPromises()

    await selectButtonByText(wrapper, 'Grok')
    await flushPromises()

    expect(wrapper.getComponent({ name: 'GroupSelector' }).props('modelValue')).toEqual([12])
    const concurrencyInput = wrapper.get('[data-testid="account-concurrency-input"]')
    expect((concurrencyInput.element as HTMLInputElement).value).toBe('2')
    expect(concurrencyInput.attributes('max')).toBe('2')
    await concurrencyInput.setValue('10')
    expect((concurrencyInput.element as HTMLInputElement).value).toBe('2')
    expect(readVisibleModelMappings(wrapper)).toEqual([
      ['grok-4.5', 'grok-4.5'],
      ['claude-opus-4-8', 'grok-4.5'],
      ['gpt-5.2', 'grok-4.5'],
      ['gpt-5.4', 'grok-4.5'],
      ['gpt-5.4-mini', 'grok-4.5'],
      ['gpt-5.5', 'grok-4.5'],
      ['gpt-5.6-luna', 'grok-4.5'],
      ['gpt-5.6-sol', 'grok-4.5'],
      ['gpt-5.6-terra', 'grok-4.5'],
    ])

    await selectButtonByText(wrapper, 'OpenAI')
    expect(readVisibleModelMappings(wrapper)).not.toContainEqual(['claude-opus-4-8', 'grok-4.5'])

    await selectButtonByText(wrapper, 'Grok')
    expect(readVisibleModelMappings(wrapper)).toEqual([
      ['grok-4.5', 'grok-4.5'],
      ['claude-opus-4-8', 'grok-4.5'],
      ['gpt-5.2', 'grok-4.5'],
      ['gpt-5.4', 'grok-4.5'],
      ['gpt-5.4-mini', 'grok-4.5'],
      ['gpt-5.5', 'grok-4.5'],
      ['gpt-5.6-luna', 'grok-4.5'],
      ['gpt-5.6-sol', 'grok-4.5'],
      ['gpt-5.6-terra', 'grok-4.5'],
    ])
  })

  it('submits the Grok group and compatibility mappings', async () => {
    const wrapper = mountModal({ groups: selectableGroups, simpleMode: false })
    await flushPromises()

    await selectButtonByText(wrapper, 'Grok')
    await selectButtonByText(wrapper, 'API Key')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('Grok account')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('xai-test')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]).toMatchObject({
      platform: 'grok',
      concurrency: 2,
      group_ids: [12],
      credentials: {
        model_mapping: {
          'grok-4.5': 'grok-4.5',
          'claude-opus-4-8': 'grok-4.5',
          'gpt-5.2': 'grok-4.5',
          'gpt-5.4': 'grok-4.5',
          'gpt-5.4-mini': 'grok-4.5',
          'gpt-5.5': 'grok-4.5',
          'gpt-5.6-luna': 'grok-4.5',
          'gpt-5.6-sol': 'grok-4.5',
          'gpt-5.6-terra': 'grok-4.5',
        },
      },
    })
  })
})

function readVisibleModelMappings(wrapper: ReturnType<typeof mountModal>) {
  const from = wrapper.findAll('input[placeholder="admin.accounts.requestModel"]')
  const to = wrapper.findAll('input[placeholder="admin.accounts.actualModel"]')
  return from.map((input, index) => [
    (input.element as HTMLInputElement).value,
    (to[index]?.element as HTMLInputElement | undefined)?.value,
  ])
}

async function selectButtonByText(wrapper: ReturnType<typeof mountModal>, text: string) {
  const button = wrapper.findAll('button').find((candidate) => candidate.text().includes(text))
  expect(button).toBeDefined()
  await button?.trigger('click')
}

async function submitApiKeyAccount(
  platform: 'openai' | 'anthropic',
  enableLongContextBilling = false,
  disableUpstreamBillingProbe = false
) {
  const wrapper = mountModal()
  await selectButtonByText(wrapper, platform === 'openai' ? 'OpenAI' : 'admin.accounts.claudeConsole')
  if (platform === 'openai') {
    await selectButtonByText(wrapper, 'API Key')
  }
  await wrapper.get('form#create-account-form input[type="text"]').setValue(`${platform} account`)
  await wrapper.get('form#create-account-form input[type="password"]').setValue('test-api-key')
  if (enableLongContextBilling) {
    await wrapper.get('[data-testid="openai-long-context-billing-toggle"]').trigger('click')
  }
  if (disableUpstreamBillingProbe) {
    await wrapper.get('[data-testid="upstream-billing-auto-probe"]').trigger('click')
  }
  await wrapper.get('form#create-account-form').trigger('submit.prevent')
  await flushPromises()
  return wrapper
}

async function openCodexImportStep(toggleClicks = 0) {
  const wrapper = mountModal()
  await selectButtonByText(wrapper, 'OpenAI')
  for (let click = 0; click < toggleClicks; click += 1) {
    await wrapper.get('[data-testid="openai-long-context-billing-toggle"]').trigger('click')
  }
  await wrapper.get('form#create-account-form input[type="text"]').setValue('Codex import')
  await wrapper.get('form#create-account-form').trigger('submit.prevent')
  return wrapper
}

function getCodexFingerprintSelect(wrapper: ReturnType<typeof mountModal>) {
  return wrapper.getComponent('[data-testid="create-codex-fingerprint-mode-select"]')
}

describe('CreateAccountModal OpenAI long-context billing', () => {
  beforeEach(() => {
    authStoreState.isSimpleMode = true
    createAccountMock.mockReset().mockResolvedValue({ id: 42, platform: 'openai', type: 'apikey' })
    probeUpstreamBillingMock.mockReset().mockResolvedValue({})
    importCodexSessionMock.mockReset().mockResolvedValue({
      created: 1,
      updated: 0,
      skipped: 0,
      failed: 0,
      errors: [],
      warnings: [],
    })
    createOpenAICodexPATMock.mockReset().mockResolvedValue({})
  })

  it('defaults new OpenAI OAuth accounts to session fingerprint convergence', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await flushPromises()

    expect(getCodexFingerprintSelect(wrapper).props('modelValue')).toBe('session')
  })

  it('offers only off and account-balanced Codex fingerprint modes', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await flushPromises()

    const options = getCodexFingerprintSelect(wrapper).props('options') as Array<{ value: string }>
    expect(options.map(option => option.value)).toEqual(['off', 'session'])
  })

  it('restores the session fingerprint default when the create dialog is reset', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    getCodexFingerprintSelect(wrapper).vm.$emit('update:modelValue', 'off')
    await flushPromises()
    expect(getCodexFingerprintSelect(wrapper).props('modelValue')).toBe('off')

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await selectButtonByText(wrapper, 'OpenAI')
    await flushPromises()

    expect(getCodexFingerprintSelect(wrapper).props('modelValue')).toBe('session')
  })

  it('submits an explicit off fingerprint mode for OpenAI OAuth imports', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    getCodexFingerprintSelect(wrapper).vm.$emit('update:modelValue', 'off')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('Codex import')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')

    wrapper.getComponent(OAuthAuthorizationFlowStub).vm.$emit('import-codex-session', 'session-json')
    await flushPromises()

    expect(importCodexSessionMock).toHaveBeenCalledTimes(1)
    expect(importCodexSessionMock.mock.calls[0]?.[0]?.extra?.codex_fingerprint_mode).toBe('off')
  })

  it('does not overwrite existing imported accounts when the fingerprint default is untouched', async () => {
    const wrapper = await openCodexImportStep()
    await wrapper.get('[data-testid="import-codex-session"]').trigger('click')
    await flushPromises()

    expect(importCodexSessionMock).toHaveBeenCalledTimes(1)
    expect(importCodexSessionMock.mock.calls[0]?.[0]?.extra?.codex_fingerprint_mode).toBeUndefined()
  })

  it('hides only the redundant account toggle when every selected group enables tier pricing', async () => {
    const wrapper = mountModal({
      groups: [
        { id: 1, long_context_pricing_enabled: true },
        { id: 2, long_context_pricing_enabled: true },
      ],
      simpleMode: false,
      stubGroupSelector: true,
    })

    await selectButtonByText(wrapper, 'OpenAI')
    await wrapper.get('[data-testid="select-pricing-groups"]').trigger('click')

    expect(wrapper.find('[data-testid="openai-long-context-billing-toggle"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="create-openai-ws-mode"]').exists()).toBe(true)
  })

  it('keeps the account toggle when any selected group disables tier pricing', async () => {
    const wrapper = mountModal({
      groups: [
        { id: 1, long_context_pricing_enabled: true },
        { id: 2, long_context_pricing_enabled: false },
      ],
      simpleMode: false,
      stubGroupSelector: true,
    })

    await selectButtonByText(wrapper, 'OpenAI')
    await wrapper.get('[data-testid="select-pricing-groups"]').trigger('click')

    expect(wrapper.find('[data-testid="openai-long-context-billing-toggle"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="create-openai-ws-mode"]').exists()).toBe(true)
  })

  it('sends false explicitly for normal OpenAI account creation by default', async () => {
    await submitApiKeyAccount('openai')

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(false)
  })

  // namespace 摊平是仅 OAuth 的兼容开关：API Key 走 chat completions 回退桥时由桥自行摊平
  it('shows the Codex namespace flatten toggle only for OpenAI OAuth accounts', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')

    expect(wrapper.find('[data-testid="create-openai-flatten-namespaces-toggle"]').exists()).toBe(
      true
    )

    await selectButtonByText(wrapper, 'API Key')
    expect(wrapper.find('[data-testid="create-openai-flatten-namespaces-toggle"]').exists()).toBe(
      false
    )
  })

  it('enables upstream billing probes by default for new OpenAI API key accounts', async () => {
    await submitApiKeyAccount('openai')

    expect(createAccountMock.mock.calls[0]?.[0]?.upstream_billing_probe_enabled).toBe(true)
  })

  it('waits for the initial upstream billing probe before refreshing the account list', async () => {
    let resolveProbe: (() => void) | undefined
    probeUpstreamBillingMock.mockImplementationOnce(
      () => new Promise<void>((resolve) => {
        resolveProbe = resolve
      })
    )

    const wrapper = await submitApiKeyAccount('openai')

    expect(probeUpstreamBillingMock).toHaveBeenCalledWith(42)
    expect(wrapper.emitted('created')).toBeUndefined()

    resolveProbe?.()
    await flushPromises()

    expect(wrapper.emitted('created')).toHaveLength(1)
  })

  it('sends an explicit disabled state when the create toggle is turned off', async () => {
    await submitApiKeyAccount('openai', false, true)

    expect(createAccountMock.mock.calls[0]?.[0]?.upstream_billing_probe_enabled).toBe(false)
    expect(probeUpstreamBillingMock).not.toHaveBeenCalled()
  })

  it('submits adaptive Kimi protocol endpoints', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Kimi')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('Kimi adaptive')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-kimi')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).toMatchObject({
      account_mode: 'payg',
      api_protocol: 'adaptive',
      base_url: 'https://api.moonshot.cn/v1',
      api_base_urls: {
        chat_completions: 'https://api.moonshot.cn/v1',
        anthropic: 'https://api.moonshot.cn/anthropic'
      }
    })
  })

  it('uses the edited adaptive Chat endpoint when previewing upstream models', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Kimi')
    await wrapper
      .get('[data-testid="cn-adaptive-base-url-chat_completions"]')
      .setValue('https://relay.example.com/v1')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-relay')

    expect(wrapper.getComponent(ModelWhitelistSelectorStub).props('syncCredentials')).toMatchObject({
      platform: 'kimi',
      type: 'apikey',
      base_url: 'https://relay.example.com/v1',
      api_key: 'sk-relay'
    })
  })

  it('exposes Agent Identity in the OpenAI authorization methods', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('OpenAI account')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')

    const flow = wrapper.getComponent(OAuthAuthorizationFlowStub)
    expect(flow.props('showManualOption')).toBe(true)
    expect(flow.props('showCodexSessionImportOption')).toBe(true)
    expect(flow.props('showAgentIdentityOption')).toBe(true)
    expect(flow.props('showCodexPatOption')).toBe(true)
    expect(flow.props('initialInputMethod')).toBe('manual')
  })

  it.each([
    ['camelCase', { authMode: 'agentIdentity', agentIdentity: { agentRuntimeId: 'runtime' } }],
    ['nested identity without auth_mode', { agent_identity: { agent_runtime_id: 'runtime' } }],
  ])('accepts backend-compatible %s Agent Identity imports', async (_name, content) => {
    const wrapper = await openCodexImportStep()
    const flow = wrapper.getComponent(OAuthAuthorizationFlowStub)
    flow.vm.inputMethod = 'agent_identity'

    flow.vm.$emit('import-codex-session', JSON.stringify(content))
    await flushPromises()

    expect(importCodexSessionMock).toHaveBeenCalledTimes(1)
  })

  it('sends true explicitly when OpenAI long-context billing is enabled', async () => {
    await submitApiKeyAccount('openai', true)

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(true)
  })

  it('omits the OpenAI setting for non-OpenAI account creation', async () => {
    await submitApiKeyAccount('anthropic')

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBeUndefined()
    // 上游倍率探测已放宽到全部 API-key 平台：非 OpenAI 平台与 OpenAI 一致，默认开启。
    expect(createAccountMock.mock.calls[0]?.[0]?.upstream_billing_probe_enabled).toBe(true)
  })

  it('sends an explicit disabled state when the non-OpenAI create toggle is turned off', async () => {
    await submitApiKeyAccount('anthropic', false, true)

    expect(createAccountMock.mock.calls[0]?.[0]?.upstream_billing_probe_enabled).toBe(false)
  })

  it('antigravity upstream 创建默认携带上游倍率探测开关', async () => {
    // antigravity upstream 走独立创建 helper，
    // 也必须与其余 API-key 平台一样默认开启探测并传递开关。
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Antigravity')
    await selectButtonByText(wrapper, 'admin.accounts.types.antigravityApikey')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('antigravity relay')
    const baseInput = wrapper
      .findAll('input')
      .find((candidate) => candidate.attributes('placeholder') === 'https://cloudcode-pa.googleapis.com')
    expect(baseInput).toBeDefined()
    await baseInput?.setValue('https://relay.example')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-upstream')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    const payload = createAccountMock.mock.calls[0]?.[0]
    expect(payload?.platform).toBe('antigravity')
    expect(payload?.type).toBe('apikey')
    expect(payload?.upstream_billing_probe_enabled).toBe(true)
    // 创建成功后前端立即发起一次首探（与其他 apikey 平台一致）。
    expect(probeUpstreamBillingMock).toHaveBeenCalledWith(42)
  })

  it('leaves Codex session import billing ownership to the backend', async () => {
    const wrapper = await openCodexImportStep()
    await wrapper.get('[data-testid="import-codex-session"]').trigger('click')
    await flushPromises()

    expect(importCodexSessionMock).toHaveBeenCalledTimes(1)
    expect(importCodexSessionMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBeUndefined()
  })

  it('leaves Codex PAT import billing ownership to the backend', async () => {
    const wrapper = await openCodexImportStep()
    await wrapper.get('[data-testid="import-codex-pat"]').trigger('click')
    await flushPromises()

    expect(createOpenAICodexPATMock).toHaveBeenCalledTimes(1)
    expect(createOpenAICodexPATMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBeUndefined()
  })

  it('sends explicit true for Codex session import after the toggle is enabled', async () => {
    const wrapper = await openCodexImportStep(1)
    await wrapper.get('[data-testid="import-codex-session"]').trigger('click')
    await flushPromises()

    expect(importCodexSessionMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(true)
  })

  it('sends explicit false for Codex session import after the toggle is changed back', async () => {
    const wrapper = await openCodexImportStep(2)
    await wrapper.get('[data-testid="import-codex-session"]').trigger('click')
    await flushPromises()

    expect(importCodexSessionMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(false)
  })

  it('sends explicit true for Codex PAT import after the toggle is enabled', async () => {
    const wrapper = await openCodexImportStep(1)
    await wrapper.get('[data-testid="import-codex-pat"]').trigger('click')
    await flushPromises()

    expect(createOpenAICodexPATMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(true)
  })

  it('sends explicit false for Codex PAT import after the toggle is changed back', async () => {
    const wrapper = await openCodexImportStep(2)
    await wrapper.get('[data-testid="import-codex-pat"]').trigger('click')
    await flushPromises()

    expect(createOpenAICodexPATMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(false)
  })
})
