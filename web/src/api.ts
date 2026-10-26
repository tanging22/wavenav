import type { DockerContainer, GalleryIcon, Settings, TGroup, TItem, User , DockerStatsResp } from './types'

export class ApiError extends Error {
  status: number
  constructor(msg: string, status = 0) {
    super(msg)
    this.status = status
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch('/api/v1' + path, {
    credentials: 'same-origin',
    cache: 'no-store',
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init.headers || {}) },
  })
  const text = await res.text()
  let data: any = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    throw new ApiError(res.status === 401 ? '请先登录' : '服务返回了无法解析的内容', res.status)
  }
  if (!res.ok || data?.ok === false) {
    throw new ApiError(data?.msg || `请求失败 (HTTP ${res.status})`, res.status)
  }
  return data as T
}

async function uploadFile(file: File, kind: 'icon' | 'wallpaper' = 'icon'): Promise<string> {
  const fd = new FormData()
  fd.append('kind', kind)
  fd.append('file', file)
  const res = await fetch('/api/v1/upload', { method: 'POST', body: fd, credentials: 'same-origin' })
  const data = await res.json().catch(() => null)
  if (!res.ok || !data?.ok) throw new ApiError(data?.msg || `上传失败 (HTTP ${res.status})`, res.status)
  return data.icon.url as string
}

export const api = {
  me: () => request<{ user: User | null; publicMode?: boolean; readonly?: boolean }>('/auth/me'),
  login: (username: string, password: string) =>
    request<{ user: User }>('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
  logout: () => request<{}>('/auth/logout', { method: 'POST' }),

  panel: () =>
    request<{
      settings: Settings
      groups: TGroup[]
      user: User | null
      readonly: boolean
    }>('/panel'),

  saveSettings: (s: Partial<Settings>) =>
    request<{ settings: Settings }>('/settings', { method: 'PUT', body: JSON.stringify(s) }),

  changeUsername: (username: string, password: string) =>
    request<{ ok: boolean; username?: string }>('/auth/username', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  changePassword: (oldPassword: string, newPassword: string) =>
    request<{}>('/auth/password', { method: 'POST', body: JSON.stringify({ old: oldPassword, new: newPassword }) }),

  createGroup: (g: Partial<TGroup>) => request<{ group: TGroup }>('/groups', { method: 'POST', body: JSON.stringify(g) }),
  updateGroup: (id: number, g: Partial<TGroup>) =>
    request<{ group: TGroup }>(`/groups/${id}`, { method: 'PUT', body: JSON.stringify(g) }),
  deleteGroup: (id: number) => request<{}>(`/groups/${id}`, { method: 'DELETE' }),

  createItem: (it: Partial<TItem>) => request<{ item: TItem }>('/items', { method: 'POST', body: JSON.stringify(it) }),
  updateItem: (id: number, it: Partial<TItem>) =>
    request<{ item: TItem }>(`/items/${id}`, { method: 'PUT', body: JSON.stringify(it) }),
  deleteItem: (id: number) => request<{}>(`/items/${id}`, { method: 'DELETE' }),
  reorder: (payload: { items?: { id: number; groupId: number; sort: number }[]; groups?: { id: number; sort: number }[] }) =>
    request<{}>('/items/reorder', { method: 'POST', body: JSON.stringify(payload) }),

  fetchSite: (url: string) =>
    request<{ title: string; icon: string }>('/fetch-site', { method: 'POST', body: JSON.stringify({ url }) }),

  upload: uploadFile,
  gallery: (opts: { kind?: 'icon' | 'wallpaper'; type?: 'image' | 'video'; page?: number; size?: number } = {}) => {
    const q = new URLSearchParams()
    if (opts.kind) q.set('kind', opts.kind)
    if (opts.type) q.set('type', opts.type)
    q.set('page', String(opts.page ?? 1))
    q.set('size', String(opts.size ?? 40))
    return request<{ total: number; page: number; size: number; list: GalleryIcon[] }>(`/gallery?${q}`)
  },
  deleteGallery: (id: number) => request<{}>(`/gallery/${id}`, { method: 'DELETE' }),

  hitokoto: (c = '') => request<{ text: string; from: string; fallback?: boolean }>(`/hitokoto?c=${encodeURIComponent(c)}`),

  docker: () =>
    request<{ available: boolean; msg?: string; containers?: DockerContainer[] }>('/docker/containers'),
  dockerStats: () =>
    request<DockerStatsResp>('/docker/stats'),
  dockerAction: (id: string, action: 'start' | 'stop' | 'restart') =>
    request<{}>(`/docker/containers/${id}/${action}`, { method: 'POST' }),
}
