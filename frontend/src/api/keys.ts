/**
 * API Keys management endpoints
 * Handles CRUD operations for user API keys
 */

import { apiClient } from './client'
import type { ApiKey, CreateApiKeyRequest, UpdateApiKeyRequest, PaginatedResponse } from '@/types'

/**
 * List all API keys for current user
 * @param page - Page number (default: 1)
 * @param pageSize - Items per page (default: 10)
 * @param filters - Optional filter parameters
 * @param options - Optional request options
 * @returns Paginated list of API keys
 */
export async function list(
  page: number = 1,
  pageSize: number = 10,
  filters?: {
    search?: string
    status?: string
    group_id?: number | string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<PaginatedResponse<ApiKey>> {
  const { data } = await apiClient.get<PaginatedResponse<ApiKey>>('/keys', {
    params: { page, page_size: pageSize, ...filters },
    signal: options?.signal
  })
  return data
}

/**
 * Get API key by ID
 * @param id - API key ID
 * @returns API key details
 */
export async function getById(id: number): Promise<ApiKey> {
  const { data } = await apiClient.get<ApiKey>(`/keys/${id}`)
  return data
}

/**
 * Create new API key
 * @param request - API key fields, including bound groups and the default group
 * @returns Created API key
 */
export async function create(request: CreateApiKeyRequest): Promise<ApiKey> {
  const payload: CreateApiKeyRequest = { name: request.name }
  if (request.group_id !== undefined) {
    payload.group_id = request.group_id
  }
  if (request.group_ids !== undefined) {
    payload.group_ids = [...new Set(request.group_ids)].sort((a, b) => a - b)
  }
  if (request.custom_key) {
    payload.custom_key = request.custom_key
  }
  if (request.ip_whitelist && request.ip_whitelist.length > 0) {
    payload.ip_whitelist = request.ip_whitelist
  }
  if (request.ip_blacklist && request.ip_blacklist.length > 0) {
    payload.ip_blacklist = request.ip_blacklist
  }
  if (request.quota !== undefined && request.quota > 0) {
    payload.quota = request.quota
  }
  if (request.expires_in_days !== undefined && request.expires_in_days > 0) {
    payload.expires_in_days = request.expires_in_days
  }
  if (request.rate_limit_5h && request.rate_limit_5h > 0) {
    payload.rate_limit_5h = request.rate_limit_5h
  }
  if (request.rate_limit_1d && request.rate_limit_1d > 0) {
    payload.rate_limit_1d = request.rate_limit_1d
  }
  if (request.rate_limit_7d && request.rate_limit_7d > 0) {
    payload.rate_limit_7d = request.rate_limit_7d
  }

  const { data } = await apiClient.post<ApiKey>('/keys', payload)
  return data
}

/**
 * Update API key
 * @param id - API key ID
 * @param updates - Fields to update
 * @returns Updated API key
 */
export async function update(id: number, updates: UpdateApiKeyRequest): Promise<ApiKey> {
  const payload = { ...updates }
  if (updates.group_ids !== undefined) {
    payload.group_ids = [...new Set(updates.group_ids)].sort((a, b) => a - b)
  }
  const { data } = await apiClient.put<ApiKey>(`/keys/${id}`, payload)
  return data
}

/**
 * Delete API key
 * @param id - API key ID
 * @returns Success confirmation
 */
export async function deleteKey(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/keys/${id}`)
  return data
}

/**
 * Toggle API key status (active/inactive)
 * @param id - API key ID
 * @param status - New status
 * @returns Updated API key
 */
export async function toggleStatus(id: number, status: 'active' | 'inactive'): Promise<ApiKey> {
  return update(id, { status })
}

export const keysAPI = {
  list,
  getById,
  create,
  update,
  delete: deleteKey,
  toggleStatus
}

export default keysAPI
