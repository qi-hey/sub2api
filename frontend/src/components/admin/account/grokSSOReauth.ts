import type { GrokSSOReauthPreviewItem } from '@/api/admin/grok'

export interface GrokSSOApplicableSelection {
  tokens: string[]
  skipped: number
}

export function selectApplicableGrokSSOTokens(
  sourceTokens: string[],
  previewItems: GrokSSOReauthPreviewItem[]
): GrokSSOApplicableSelection {
  const selectedIndexes = new Set<number>()
  const tokens: string[] = []

  for (const item of previewItems) {
    const sourceIndex = item.index - 1
    if (
      item.action !== 'update' ||
      item.error ||
      sourceIndex < 0 ||
      sourceIndex >= sourceTokens.length ||
      selectedIndexes.has(sourceIndex)
    ) {
      continue
    }
    selectedIndexes.add(sourceIndex)
    tokens.push(sourceTokens[sourceIndex])
  }

  return {
    tokens,
    skipped: Math.max(0, sourceTokens.length - tokens.length)
  }
}
