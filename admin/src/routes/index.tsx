import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState, type ChangeEvent } from 'react'
import type { User } from 'oidc-client-ts'
import { getUser, login, logout } from '../lib/auth'
import { api, type ConvertJob } from '../lib/api'

export const Route = createFileRoute('/')({ component: Dashboard })

function Dashboard() {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [job, setJob] = useState<ConvertJob | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    getUser()
      .then(setUser)
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <p>Loading…</p>

  if (!user || user.expired) {
    return (
      <div>
        <p>You are not signed in.</p>
        <button onClick={() => login()}>Sign in with Ory-style OIDC</button>
      </div>
    )
  }

  async function onUpload(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setBusy(true)
    try {
      const { sourceKey } = await api.upload(file)
      const { job } = await api.convert({ sourceKey })
      setJob(job)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <p>
        Signed in as <strong>{user.profile.email ?? user.profile.sub}</strong>{' '}
        <button onClick={() => logout()}>Sign out</button>
      </p>

      <h2>Convert an archive (CBZ / EPUB / PDF → AVIF)</h2>
      <input type="file" accept=".cbz,.epub,.pdf" onChange={onUpload} disabled={busy} />
      {busy && <p>Uploading & converting…</p>}
      {job && (
        <pre style={{ background: '#f5f5f5', padding: 12 }}>
          {JSON.stringify(job, null, 2)}
        </pre>
      )}
    </div>
  )
}
