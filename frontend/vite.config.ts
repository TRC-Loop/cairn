import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			'/api': 'http://localhost:8080',
			'/p': 'http://localhost:8080',
			'/push': 'http://localhost:8080',
			'/healthz': 'http://localhost:8080',
			'/readyz': 'http://localhost:8080',
			'/static': 'http://localhost:8080'
		}
	}
});
