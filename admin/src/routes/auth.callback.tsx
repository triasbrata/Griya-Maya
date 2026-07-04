import { createFileRoute, useRouter } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { handleCallback } from '../lib/auth'

// OIDC redirect target: exchanges the authorization code (PKCE) for tokens,
// then returns to the dashboard.
export const Route = createFileRoute('/auth/callback')({ component: Callback })

function Callback() {
  const router = useRouter()
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    handleCallback()
      .then(() => router.navigate({ to: '/' }))
      .catch((e) => setError(String(e)))
  }, [router])

  if (error) return <p style={{ color: 'crimson' }}>Sign-in failed: {error}</p>
  return <p>Completing sign-in…</p>
}
