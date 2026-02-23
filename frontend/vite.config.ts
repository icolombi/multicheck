import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
	// Tailwind v4: use the Vite plugin instead of PostCSS for better performance
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			'/api': {
				target: 'http://localhost:8080',
				changeOrigin: true,
				rewrite: (path) => path.replace(/^\/api/, '')
			}
		}
	}
});
