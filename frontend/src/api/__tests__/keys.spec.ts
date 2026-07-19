import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post, put } = vi.hoisted(() => ({
  post: vi.fn(),
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { post, put },
}))

import { create, update } from '@/api/keys'

describe('API key API', () => {
  beforeEach(() => {
    post.mockReset()
    put.mockReset()
    post.mockResolvedValue({ data: { id: 1, key: 'sk-test' } })
    put.mockResolvedValue({ data: { id: 1, key: 'sk-test' } })
  })

  it('sends stable group bindings with the default group', async () => {
    await create({
      name: 'CC Switch',
      group_id: 2,
      group_ids: [12, 2, 11, 12],
    })

    expect(post).toHaveBeenCalledWith('/keys', {
      name: 'CC Switch',
      group_id: 2,
      group_ids: [2, 11, 12],
    })
  })

  it('normalizes group bindings on update', async () => {
    await update(1, {
      group_id: 2,
      group_ids: [12, 2, 11, 12],
    })

    expect(put).toHaveBeenCalledWith('/keys/1', {
      group_id: 2,
      group_ids: [2, 11, 12],
    })
  })
})
