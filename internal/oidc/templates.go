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
</body>
</html>{{end}}

{{define "card"}}{{if .Pending}}{{template "pending" .}}{{else if .Consent}}{{template "consent" .}}{{else}}{{template "login" .}}{{end}}{{end}}

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
{{end}}

{{define "consent"}}
  <div class="space-y-1.5">
    <h1 class="text-2xl font-semibold tracking-tight">Authorize {{.ClientName}}</h1>
    <p class="text-sm text-muted-foreground">is requesting access to your account.</p>
  </div>
  <ul class="mt-6 space-y-2">
    {{range .Scopes}}<li class="rounded-md border border-border bg-muted/50 px-3 py-2 text-sm">{{.}}</li>{{end}}
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
{{end}}
`))

// loginData drives the login, consent, and pending-review renders.
type loginData struct {
	ID         string
	Email      string
	Error      string
	Consent    bool
	Pending    bool
	ClientName string
	Scopes     []string
}
