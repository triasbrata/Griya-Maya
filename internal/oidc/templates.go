package oidc

import "html/template"

// loginTemplates holds the server-rendered login + consent pages. htmx is
// loaded from a CDN; forms use hx-post and swap the returned partial into #card.
var loginTemplates = template.Must(template.New("").Parse(`
{{define "page"}}<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Mihon — Sign in</title>
  <script src="https://unpkg.com/htmx.org@1.9.12"></script>
  <style>
    :root { color-scheme: light dark; }
    body { font-family: system-ui, -apple-system, Segoe UI, Roboto, sans-serif;
           display: flex; align-items: center; justify-content: center;
           min-height: 100vh; margin: 0; background: #0f1115; color: #e7e9ee; }
    #card { width: 320px; padding: 28px; border-radius: 12px; background: #1a1d24;
            box-shadow: 0 10px 30px rgba(0,0,0,.4); }
    h1 { font-size: 1.25rem; margin: 0 0 20px; }
    label { display: block; font-size: .8rem; margin: 12px 0 4px; color: #9aa0aa; }
    input { width: 100%; box-sizing: border-box; padding: 10px; border-radius: 8px;
            border: 1px solid #2b2f3a; background: #0f1115; color: #e7e9ee; }
    button { margin-top: 18px; width: 100%; padding: 11px; border: 0; border-radius: 8px;
             background: #4f7cff; color: #fff; font-weight: 600; cursor: pointer; }
    button.secondary { background: #2b2f3a; margin-top: 8px; }
    .err { color: #ff6b6b; font-size: .8rem; min-height: 1rem; margin-top: 10px; }
    .scopes { list-style: none; padding: 0; margin: 8px 0 0; }
    .scopes li { padding: 6px 0; font-size: .85rem; border-bottom: 1px solid #2b2f3a; }
    .muted { color: #9aa0aa; font-size: .8rem; }
  </style>
</head>
<body>
  <div id="card">{{template "card" .}}</div>
</body>
</html>{{end}}

{{define "card"}}{{if .Consent}}{{template "consent" .}}{{else}}{{template "login" .}}{{end}}{{end}}

{{define "login"}}
  <h1>Sign in</h1>
  <form hx-post="/login/username" hx-target="#card" hx-swap="innerHTML">
    <input type="hidden" name="id" value="{{.ID}}">
    <label for="email">Email</label>
    <input id="email" name="email" type="email" autocomplete="username" autofocus>
    <label for="password">Password</label>
    <input id="password" name="password" type="password" autocomplete="current-password">
    <div class="err">{{.Error}}</div>
    <button type="submit">Continue</button>
  </form>
{{end}}

{{define "consent"}}
  <h1>Authorize {{.ClientName}}</h1>
  <p class="muted">This application is requesting access to:</p>
  <ul class="scopes">
    {{range .Scopes}}<li>{{.}}</li>{{end}}
  </ul>
  <form hx-post="/login/consent" hx-target="#card" hx-swap="innerHTML">
    <input type="hidden" name="id" value="{{.ID}}">
    <div class="err">{{.Error}}</div>
    <button type="submit" name="action" value="approve">Allow</button>
    <button type="submit" name="action" value="deny" class="secondary">Deny</button>
  </form>
{{end}}
`))

// loginData drives both the login and consent renders.
type loginData struct {
	ID         string
	Error      string
	Consent    bool
	ClientName string
	Scopes     []string
}
