// Thin client for the manga-server API. Attaches the OIDC access token as a
// Bearer credential on every request.
import { getAccessToken } from './auth'

const BASE = (import.meta.env.VITE_API_BASE as string) ?? ''

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = await getAccessToken()
  const headers = new Headers(init.headers)
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const res = await fetch(BASE + path, { ...init, headers })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.status === 204 ? (undefined as T) : ((await res.json()) as T)
}

export interface ConvertJob {
  id: string
  sourceKey: string
  format: string
  status: string
  pageCount: number
  error?: string
}

export const api = {
  // Kick off a conversion of an archive already in R2.
  convert: (body: { sourceKey: string; chapterId?: string; format?: string }) =>
    request<{ job: ConvertJob }>('/v1/convert', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }),

  // Poll a conversion job.
  job: (id: string) => request<ConvertJob>(`/v1/convert/jobs/${id}`),

  // Upload an archive (multipart). Returns the stored R2 key.
  upload: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request<{ sourceKey: string }>('/v1/convert/upload', {
      method: 'POST',
      body: form,
    })
  },
}
