import { defineConfig } from 'vite'
import { tanstackStart } from '@tanstack/react-start/plugin/vite'
import viteReact from '@vitejs/plugin-react'

export default defineConfig({
  server: { port: 3000 },
  plugins: [
    // Deploy target: set to 'cloudflare-module' when shipping to Cloudflare
    // Workers/Pages. Leave default for local dev / node output.
    tanstackStart({ target: process.env.DEPLOY_TARGET as any }),
    viteReact(),
  ],
})
