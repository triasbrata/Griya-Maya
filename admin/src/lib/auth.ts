// OIDC Relying Party for the admin app.
//
// Uses authorization_code + PKCE against the embedded OpenID Provider in
// manga-server. All calls are browser-only (oidc-client-ts uses window/crypto),
// so the UserManager is created lazily on the client.
import { UserManager, WebStorageStateStore, type User } from 'oidc-client-ts'

let manager: UserManager | null = null

export function getUserManager(): UserManager {
  if (typeof window === 'undefined') {
    throw new Error('OIDC UserManager is only available in the browser')
  }
  if (!manager) {
    manager = new UserManager({
      // `authority` is the IdP issuer; endpoints are discovered from
      // {authority}/.well-known/openid-configuration.
      authority: import.meta.env.VITE_OIDC_ISSUER as string,
      client_id: (import.meta.env.VITE_OIDC_CLIENT_ID as string) ?? 'admin-web',
      redirect_uri: window.location.origin + '/auth/callback',
      post_logout_redirect_uri: window.location.origin,
      response_type: 'code',
      scope:
        (import.meta.env.VITE_OIDC_SCOPE as string) ??
        'openid profile email offline_access manga.write',
      // Public PKCE client — no secret in the browser.
      userStore: new WebStorageStateStore({ store: window.localStorage }),
      automaticSilentRenew: true,
    })
  }
  return manager
}

export const login = () => getUserManager().signinRedirect()
export const handleCallback = () => getUserManager().signinRedirectCallback()
export const logout = () => getUserManager().signoutRedirect()
export const getUser = (): Promise<User | null> => getUserManager().getUser()

export async function getAccessToken(): Promise<string | null> {
  const user = await getUser()
  return user && !user.expired ? user.access_token : null
}
