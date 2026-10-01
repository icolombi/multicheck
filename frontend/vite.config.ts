import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';
import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
	// Tailwind v4: use the Vite plugin instead of PostCSS for better performance
	plugins: [
		tailwindcss(),
		sveltekit({
			preprocess: vitePreprocess(),
			// SPA: static build with client-side routing, served by nginx (see Dockerfile)
			adapter: adapter({
				fallback: 'index.html'
			})
		})
	],
	server: {
		proxy: {
			'/api': {
				target: 'http://localhost:8080',
				changeOrigin: true,
				rewrite: (path) => path.replace(/^\/api/, '')
			}
		}
	},
	preview: {
		// Allow Traefik reverse proxy host
		allowedHosts: ['lab.icolombi.net']
	}
});
