import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	build: {
		// Inline assets under 100KB as data URIs to reduce HTTP round-trips
		// from the webview to the embedded localhost server.
		assetsInlineLimit: 100_000
	}
});
