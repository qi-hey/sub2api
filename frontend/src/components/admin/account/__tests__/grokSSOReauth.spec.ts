import { describe, expect, it } from 'vitest'
import { selectApplicableGrokSSOTokens } from '../grokSSOReauth'

describe('Grok SSO reauthentication selection', () => {
  it('keeps the apply selection empty before preview', () => {
    expect(selectApplicableGrokSSOTokens(['a@example.com:token-a'], [])).toEqual({
      tokens: [],
      skipped: 1
    })
  })

  it('submits only valid existing-account matches and skips the rest', () => {
    const selection = selectApplicableGrokSSOTokens(
      ['a@example.com:token-a', 'b@example.com:token-b', 'c@example.com:token-c'],
      [
        { index: 1, action: 'update', account_id: 10 },
        { index: 2, action: 'unmatched', error: 'not found' },
        { index: 3, action: 'conflict', error: 'duplicate account' }
      ]
    )

    expect(selection).toEqual({
      tokens: ['a@example.com:token-a'],
      skipped: 2
    })
  })

  it('ignores duplicate and out-of-range preview indexes', () => {
    const selection = selectApplicableGrokSSOTokens(['a@example.com:token-a'], [
      { index: 1, action: 'update' },
      { index: 1, action: 'update' },
      { index: 2, action: 'update' }
    ])

    expect(selection).toEqual({ tokens: ['a@example.com:token-a'], skipped: 0 })
  })
})
