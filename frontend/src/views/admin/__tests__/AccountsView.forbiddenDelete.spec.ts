import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getUpstreamBillingProbeSettings,
  getAllProxies,
  getAllGroups,
  bulkDeleteForbidden
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getUpstreamBillingProbeSettings: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  bulkDeleteForbidden: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      getUpstreamBillingProbeSettings,
      bulkDeleteForbidden,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      probeUpstreamBillingBatch: vi.fn(),
      toggleSchedulable: vi.fn()
    },
    proxies: { getAll: getAllProxies },
    groups: { getAll: getAllGroups }
  }
}))

const showError = vi.fn()
const showSuccess = vi.fn()

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess, showInfo: vi.fn() })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token' })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => `${key}:${params?.count ?? ''}` })
  }
})

const AccountTableFiltersStub = {
  props: ['filters', 'searchQuery'],
  emits: ['update:filters', 'update:searchQuery', 'change'],
  template: `
    <button
      data-test="apply-forbidden"
      @click="$emit('update:filters', { ...filters, platform: 'grok', status: 'forbidden' }); $emit('change')"
    >filter</button>
  `
}

const mountAccountsView = () => mount(AccountsView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
      AccountTableFilters: AccountTableFiltersStub,
      AccountTableActions: { template: '<div><slot name="after" /></div>' },
      DataTable: true,
      Pagination: true,
      ConfirmDialog: true,
      AccountActionMenu: true,
      ImportDataModal: true,
      ReAuthAccountModal: true,
      AccountTestModal: true,
      AccountStatsModal: true,
      ScheduledTestsPanel: true,
      SyncFromCrsModal: true,
      TempUnschedStatusModal: true,
      ErrorPassthroughRulesModal: true,
      TLSFingerprintProfilesModal: true,
      CreateAccountModal: true,
      EditAccountModal: true,
      BulkEditAccountModal: true,
      PlatformTypeBadge: true,
      AccountCapacityCell: true,
      AccountStatusIndicator: true,
      AccountTodayStatsCell: true,
      AccountGroupsCell: true,
      AccountUsageCell: true,
      Icon: true
    }
  }
})

const applyForbiddenFilter = async (wrapper: ReturnType<typeof mountAccountsView>) => {
  await flushPromises()
  await wrapper.get('[data-test="apply-forbidden"]').trigger('click')
  await vi.advanceTimersByTimeAsync(350)
  await flushPromises()
}

describe('admin AccountsView Forbidden bulk deletion', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    localStorage.clear()
    showError.mockReset()
    showSuccess.mockReset()
    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getUpstreamBillingProbeSettings.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()
    bulkDeleteForbidden.mockReset()

    listAccounts.mockImplementation(async (_page, _pageSize, filters) => {
      if (filters?.status === 'forbidden') {
        return { items: [{ id: 1542, name: 'forbidden', platform: 'grok', type: 'oauth', status: 'active' }], total: 1074, page: 1, page_size: 20, pages: 54 }
      }
      return { items: [], total: 0, page: 1, page_size: 20, pages: 0 }
    })
    listWithEtag.mockResolvedValue({ notModified: true, etag: null, data: null })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getUpstreamBillingProbeSettings.mockResolvedValue({ enabled: true, interval_minutes: 30 })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
    bulkDeleteForbidden.mockResolvedValue({ success: 1074, failed: 0, success_ids: [], failed_ids: [] })
    vi.stubGlobal('confirm', vi.fn(() => true))
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('deletes every server-filtered Forbidden account in one protected request', async () => {
    const wrapper = mountAccountsView()

    expect(wrapper.find('[data-test="delete-all-forbidden"]').exists()).toBe(false)
    await applyForbiddenFilter(wrapper)

    expect(listAccounts).toHaveBeenCalledWith(
      1,
      expect.any(Number),
      expect.objectContaining({ platform: 'grok', status: 'forbidden' }),
      expect.any(Object)
    )
    await wrapper.get('[data-test="delete-all-forbidden"]').trigger('click')
    await flushPromises()

    expect(bulkDeleteForbidden).toHaveBeenCalledWith({
      filters: {
        platform: 'grok',
        type: '',
        status: 'forbidden',
        group: '',
        search: '',
        privacy_mode: ''
      },
      expected_count: 1074
    })
    expect(showSuccess).toHaveBeenCalled()
  })

  it('refreshes the list instead of deleting when the server count changed', async () => {
    bulkDeleteForbidden.mockRejectedValueOnce({ status: 409, message: 'count changed' })
    const wrapper = mountAccountsView()
    await applyForbiddenFilter(wrapper)

    await wrapper.get('[data-test="delete-all-forbidden"]').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith(
      expect.stringContaining('admin.accounts.bulkActions.deleteAllForbiddenCountChanged')
    )
    expect(listAccounts).toHaveBeenCalledTimes(3)
  })

  it('reports partial failures without claiming full success', async () => {
    bulkDeleteForbidden.mockResolvedValueOnce({
      success: 1000,
      failed: 74,
      success_ids: [],
      failed_ids: []
    })
    const wrapper = mountAccountsView()
    await applyForbiddenFilter(wrapper)

    await wrapper.get('[data-test="delete-all-forbidden"]').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith(
      expect.stringContaining('admin.accounts.bulkActions.deleteAllForbiddenPartial')
    )
    expect(showSuccess).not.toHaveBeenCalled()
  })
})
