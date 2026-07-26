import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const accountsSource = readFileSync(resolve(process.cwd(), 'src/views/admin/AccountsView.vue'), 'utf8')
const modalSource = readFileSync(
  resolve(process.cwd(), 'src/components/admin/account/GrokSSOReauthModal.vue'),
  'utf8'
)

describe('AccountsView Grok SSO reauthentication', () => {
  it('exposes the batch reauthentication entry in the account tools menu', () => {
    expect(accountsSource).toContain("t('admin.accounts.grokSSOReauth.menu')")
    expect(accountsSource).toContain('@click="openGrokSSOReauth"')
    expect(accountsSource).toContain('<GrokSSOReauthModal')
  })

  it('requires a preview before applying uploaded SSO tokens', () => {
    expect(modalSource).toContain('type="file"')
    expect(modalSource).toContain('preview: true')
    expect(modalSource).toContain('confirmed: true')
    expect(modalSource).toContain(':disabled="busy || !canApply"')
    expect(modalSource).toContain('sso_tokens: applicableTokens')
    expect(modalSource).toContain('previewRequired')
  })
})
