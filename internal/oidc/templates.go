package oidc

import "html/template"

// loginTemplates holds the server-rendered login + consent pages. htmx is
// loaded from a CDN; forms use hx-post and swap the returned partial into #card.
// Styling replicates shadcn/ui's default auth aesthetic via the Tailwind Play
// CDN plus shadcn's CSS-variable design tokens (light + dark).
var loginTemplates = template.Must(template.New("").Parse(`
{{define "page"}}<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Griyamedia — Sign in</title>
  <style>
    :root {
      --background: 0 0% 100%;
      --foreground: 240 10% 3.9%;
      --card: 0 0% 100%;
      --card-foreground: 240 10% 3.9%;
      --primary: 240 5.9% 10%;
      --primary-foreground: 0 0% 98%;
      --muted: 240 4.8% 95.9%;
      --muted-foreground: 240 3.8% 46.1%;
      --accent: 240 4.8% 95.9%;
      --accent-foreground: 240 5.9% 10%;
      --border: 240 5.9% 90%;
      --input: 240 5.9% 90%;
      --ring: 240 10% 3.9%;
      --destructive: 0 84.2% 60.2%;
    }
    @media (prefers-color-scheme: dark) {
      :root {
        --background: 240 10% 3.9%;
        --foreground: 0 0% 98%;
        --card: 240 10% 3.9%;
        --card-foreground: 0 0% 98%;
        --primary: 0 0% 98%;
        --primary-foreground: 240 5.9% 10%;
        --muted: 240 3.7% 15.9%;
        --muted-foreground: 240 5% 64.9%;
        --accent: 240 3.7% 15.9%;
        --accent-foreground: 0 0% 98%;
        --border: 240 3.7% 15.9%;
        --input: 240 3.7% 15.9%;
        --ring: 240 4.9% 83.9%;
        --destructive: 0 62.8% 30.6%;
      }
    }
  </style>
  <script>
    window.tailwind = {
      config: {
        theme: {
          extend: {
            colors: {
              background: 'hsl(var(--background))',
              foreground: 'hsl(var(--foreground))',
              card: { DEFAULT: 'hsl(var(--card))', foreground: 'hsl(var(--card-foreground))' },
              primary: { DEFAULT: 'hsl(var(--primary))', foreground: 'hsl(var(--primary-foreground))' },
              muted: { DEFAULT: 'hsl(var(--muted))', foreground: 'hsl(var(--muted-foreground))' },
              accent: { DEFAULT: 'hsl(var(--accent))', foreground: 'hsl(var(--accent-foreground))' },
              border: 'hsl(var(--border))',
              input: 'hsl(var(--input))',
              ring: 'hsl(var(--ring))',
              destructive: 'hsl(var(--destructive))',
            },
          },
        },
      },
    };
  </script>
  <script src="https://cdn.tailwindcss.com"></script>
  <script src="https://unpkg.com/htmx.org@1.9.12"></script>
</head>
<body class="min-h-screen bg-background text-foreground antialiased">
  <div class="flex min-h-screen flex-col items-center justify-center px-4 py-10">
    <div class="mb-6 flex items-center gap-2 text-lg font-semibold tracking-tight">
      <span class="inline-flex h-7 w-7 items-center justify-center rounded-md bg-primary text-primary-foreground text-sm font-bold">G</span>
      <span>Griyamedia</span>
    </div>
    <div id="card" class="w-full max-w-sm rounded-xl border border-border bg-card text-card-foreground shadow-sm p-6 sm:p-8">{{template "card" .}}</div>
  </div>
  <script>
  (function(){
    function b64urlToBuf(s){
      s = s.replace(/-/g,'+').replace(/_/g,'/');
      var pad = s.length % 4; if(pad){ s += '===='.slice(pad); }
      var bin = atob(s); var buf = new Uint8Array(bin.length);
      for(var i=0;i<bin.length;i++){ buf[i] = bin.charCodeAt(i); }
      return buf.buffer;
    }
    function bufToB64url(buf){
      var bytes = new Uint8Array(buf); var bin = '';
      for(var i=0;i<bytes.length;i++){ bin += String.fromCharCode(bytes[i]); }
      return btoa(bin).replace(/\+/g,'-').replace(/\//g,'_').replace(/=+$/,'');
    }
    async function passkeyLogin(id, errEl){
      if(errEl){ errEl.textContent = ''; }
      try{
        var begin = await fetch('/login/webauthn/begin?authRequestID=' + encodeURIComponent(id), { method: 'POST' });
        var beginData = await begin.json();
        if(!begin.ok){ throw new Error(beginData.error || 'Could not start passkey sign-in'); }
        var options = beginData.publicKey;
        options.challenge = b64urlToBuf(options.challenge);
        if(options.allowCredentials){
          options.allowCredentials = options.allowCredentials.map(function(c){ c.id = b64urlToBuf(c.id); return c; });
        }
        var assertion = await navigator.credentials.get({ publicKey: options });
        var payload = {
          id: assertion.id,
          rawId: bufToB64url(assertion.rawId),
          type: assertion.type,
          response: {
            authenticatorData: bufToB64url(assertion.response.authenticatorData),
            clientDataJSON: bufToB64url(assertion.response.clientDataJSON),
            signature: bufToB64url(assertion.response.signature),
            userHandle: assertion.response.userHandle ? bufToB64url(assertion.response.userHandle) : null
          }
        };
        var finish = await fetch('/login/webauthn/finish?authRequestID=' + encodeURIComponent(id), {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload)
        });
        var data = await finish.json();
        if(!finish.ok){ throw new Error(data.error || 'Passkey verification failed'); }
        if(data.redirect){ window.location = data.redirect; }
      }catch(err){ if(errEl){ errEl.textContent = err.message || 'Passkey sign-in failed'; } }
    }
    document.addEventListener('click', function(e){
      var btn = e.target.closest ? e.target.closest('#passkey-btn') : null;
      if(!btn){ return; }
      e.preventDefault();
      passkeyLogin(btn.getAttribute('data-auth-id'), document.getElementById('passkey-error'));
    });
  })();
  </script>
</body>
</html>{{end}}

{{define "card"}}{{if .Success}}{{template "success" .}}{{else if .Register}}{{template "register" .}}{{else if .Pending}}{{template "pending" .}}{{else if .Consent}}{{template "consent" .}}{{else}}{{template "login" .}}{{end}}{{end}}

{{define "pending"}}
  <div class="flex flex-col items-center text-center">
    <span class="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-full bg-muted text-muted-foreground">
      <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><polyline points="12 6 12 12 16 14"></polyline></svg>
    </span>
    <h1 class="text-2xl font-semibold tracking-tight">Account under review</h1>
    <p class="mt-2 text-sm text-muted-foreground">
      {{if .Email}}Your email <span class="font-medium text-foreground">{{.Email}}</span> is being reviewed by the server owner.{{else}}Your account is being reviewed by the server owner.{{end}}
      You'll be able to sign in once it's approved.
    </p>
    <a href="/login/username?authRequestID={{.ID}}"
      class="mt-6 inline-flex h-9 w-full items-center justify-center rounded-md border border-input bg-background text-sm font-medium shadow-sm transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
      Back to sign in
    </a>
  </div>
{{end}}

{{define "login"}}
  <div class="space-y-1.5">
    <h1 class="text-2xl font-semibold tracking-tight">Sign in</h1>
    <p class="text-sm text-muted-foreground">Enter your email and password to continue.</p>
  </div>
  <form hx-post="/login/username" hx-target="#card" hx-swap="innerHTML" class="mt-6 space-y-4">
    <input type="hidden" name="id" value="{{.ID}}">
    <div class="space-y-2">
      <label for="email" class="text-sm font-medium leading-none">Email</label>
      <input id="email" name="email" type="email" autocomplete="username" autofocus
        class="h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
    </div>
    <div class="space-y-2">
      <label for="password" class="text-sm font-medium leading-none">Password</label>
      <input id="password" name="password" type="password" autocomplete="current-password"
        class="h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
    </div>
    <div class="min-h-[1.25rem] text-sm text-destructive">{{.Error}}</div>
    <button type="submit"
      class="inline-flex h-9 w-full items-center justify-center rounded-md bg-primary text-sm font-medium text-primary-foreground shadow transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
      Continue
    </button>
  </form>
  {{if .WebAuthn}}
  <div class="mt-4">
    <div class="relative flex items-center justify-center text-xs uppercase text-muted-foreground">
      <span class="bg-card px-2">or</span>
    </div>
    <button type="button" id="passkey-btn" data-auth-id="{{.ID}}"
      class="mt-3 inline-flex h-9 w-full items-center justify-center gap-2 rounded-md border border-input bg-background text-sm font-medium shadow-sm transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
      <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="11" x="3" y="11" rx="2" ry="2"></rect><path d="M7 11V7a5 5 0 0 1 10 0v4"></path></svg>
      Sign in with passkey
    </button>
    <div id="passkey-error" class="mt-2 min-h-[1.25rem] text-center text-sm text-destructive"></div>
  </div>
  {{end}}
  <p class="mt-4 text-center text-sm text-muted-foreground">
    Don't have an account?
    <a href="/register?authRequestID={{.ID}}" class="font-medium text-foreground underline-offset-4 hover:underline">Register</a>
  </p>
{{end}}

{{define "register"}}
  <div class="space-y-1.5">
    <h1 class="text-2xl font-semibold tracking-tight">Create an account</h1>
    <p class="text-sm text-muted-foreground">Have an invite code? You can sign in right away. Without one, an admin approves your account first.</p>
  </div>
  <form hx-post="/register" hx-target="#card" hx-swap="innerHTML" class="mt-6 space-y-4">
    <input type="hidden" name="id" value="{{.ID}}">
    <div class="space-y-2">
      <label for="code" class="text-sm font-medium leading-none">Invite code <span class="font-normal text-muted-foreground">(optional)</span></label>
      <input id="code" name="code" type="text" value="{{.Code}}" autocomplete="off"
        class="h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
    </div>
    <div class="space-y-2">
      <label for="reg-name" class="text-sm font-medium leading-none">Name</label>
      <input id="reg-name" name="name" type="text" value="{{.Name}}" autocomplete="name" autofocus
        class="h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
    </div>
    <div class="space-y-2">
      <label for="reg-email" class="text-sm font-medium leading-none">Email</label>
      <input id="reg-email" name="email" type="email" value="{{.Email}}" autocomplete="username"
        class="h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
    </div>
    <div class="space-y-2">
      <label for="reg-password" class="text-sm font-medium leading-none">Password</label>
      <input id="reg-password" name="password" type="password" autocomplete="new-password"
        class="h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
      <p class="text-xs text-muted-foreground">At least 8 characters.</p>
    </div>
    <div class="min-h-[1.25rem] text-sm text-destructive">{{.Error}}</div>
    <button type="submit"
      class="inline-flex h-9 w-full items-center justify-center rounded-md bg-primary text-sm font-medium text-primary-foreground shadow transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
      Create account
    </button>
  </form>
  <p class="mt-4 text-center text-sm text-muted-foreground">
    Already have an account?
    <a href="/login/username?authRequestID={{.ID}}" class="font-medium text-foreground underline-offset-4 hover:underline">Sign in</a>
  </p>
{{end}}

{{define "success"}}
  <div class="flex flex-col items-center text-center">
    <span class="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-full bg-muted text-muted-foreground">
      <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline></svg>
    </span>
    {{if .Verified}}
      <h1 class="text-2xl font-semibold tracking-tight">You're all set</h1>
      <p class="mt-2 text-sm text-muted-foreground">
        Your account {{if .Email}}<span class="font-medium text-foreground">{{.Email}}</span> {{end}}is ready. You can sign in now.
      </p>
    {{else}}
      <h1 class="text-2xl font-semibold tracking-tight">Account created</h1>
      <p class="mt-2 text-sm text-muted-foreground">
        Your account {{if .Email}}<span class="font-medium text-foreground">{{.Email}}</span> {{end}}was created and is awaiting approval by the server owner. You'll be able to sign in once it's approved.
      </p>
    {{end}}
    <a href="/login/username?authRequestID={{.ID}}"
      class="mt-6 inline-flex h-9 w-full items-center justify-center rounded-md border border-input bg-background text-sm font-medium shadow-sm transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
      {{if .Verified}}Sign in{{else}}Back to sign in{{end}}
    </a>
  </div>
{{end}}

{{define "consent"}}
  <div class="flex flex-col items-center text-center">
    <span class="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-full bg-muted text-muted-foreground">
      <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="11" x="3" y="11" rx="2" ry="2"></rect><path d="M7 11V7a5 5 0 0 1 10 0v4"></path></svg>
    </span>
    <h1 class="text-2xl font-semibold tracking-tight">Authorize {{.ClientName}}</h1>
    <p class="mt-2 text-sm text-muted-foreground"><span class="font-medium text-foreground">{{.ClientName}}</span> wants permission to:</p>
  </div>
  <ul class="mt-6 space-y-2">
    {{range .Scopes}}
    <li class="flex items-start gap-3 rounded-md border border-border bg-muted/40 px-3 py-2.5">
      <span class="mt-0.5 inline-flex h-4 w-4 shrink-0 items-center justify-center text-muted-foreground">
        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
      </span>
      <div class="min-w-0">
        <div class="text-sm font-medium leading-tight text-foreground">{{.Label}}</div>
        <div class="text-xs text-muted-foreground">{{.Name}}</div>
      </div>
    </li>
    {{end}}
  </ul>
  <form hx-post="/login/consent" hx-target="#card" hx-swap="innerHTML" class="mt-6 space-y-3">
    <input type="hidden" name="id" value="{{.ID}}">
    <div class="min-h-[1.25rem] text-sm text-destructive">{{.Error}}</div>
    <button type="submit" name="action" value="approve"
      class="inline-flex h-9 w-full items-center justify-center rounded-md bg-primary text-sm font-medium text-primary-foreground shadow transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
      Allow
    </button>
    <button type="submit" name="action" value="deny"
      class="inline-flex h-9 w-full items-center justify-center rounded-md border border-input bg-background text-sm font-medium shadow-sm transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
      Deny
    </button>
  </form>
  <p class="mt-4 text-center text-xs text-muted-foreground">You won't be asked again for these permissions.</p>
{{end}}
`))

// loginData drives the login, register, consent, pending-review, and
// registration-success renders.
type loginData struct {
	ID         string
	Email      string
	Name       string
	Code       string
	Error      string
	Consent    bool
	Pending    bool
	Register   bool
	Success    bool
	Verified   bool
	ClientName string
	Scopes     []scopeView
	// WebAuthn enables the "Sign in with passkey" button on the login card.
	WebAuthn bool
}
