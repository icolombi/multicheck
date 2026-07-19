import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),

	kit: {
		// SPA: static build with client-side routing, served by nginx (see Dockerfile)
		adapter: adapter({
			fallback: 'index.html'
		})
	}
};

export default config;
