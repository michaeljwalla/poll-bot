import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

// The prefix the app is mounted at, baked in at build time. Go reads the same
// WEB_ROOT_PATH at runtime (see main.go webRootPath) so the two always agree —
// a mismatch means every asset and API call 404s.
const raw = (process.env.WEB_ROOT_PATH ?? '').trim().replace(/\/+$/, '');
const base: '' | `/${string}` =
	raw === '' ? '' : raw.startsWith('/') ? (raw as `/${string}`) : `/${raw}`;

// Go embeds this directory, so the build has to land inside the Go module.
const OUT_DIR = process.env.OUTPUT_DIR ?? './dist/';

const API_TARGET = 'http://localhost:10000';

export default defineConfig({
	plugins: [
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},

			// A pure SPA: every route is auth-gated and client-rendered, so there
			// is nothing to prerender and no Node process to run beside the Go
			// binary. `fallback` is the shell the Go handler serves for any path
			// the client router owns.
			adapter: adapter({
				pages: OUT_DIR,
				assets: OUT_DIR,
				fallback: 'index.html',
				// Cloudflare compresses at the edge and Go<->cloudflared is
				// loopback, so shipping .br/.gz copies would only bloat the binary.
				precompress: false
			}),
			paths: { base }
		})
	],
	server: {
		proxy: {
			// matches the prefix the client actually requests under `base`
			[`${base}/api`]: {
				target: API_TARGET,
				changeOrigin: true,
				secure: false
			}
		}
	}
});
