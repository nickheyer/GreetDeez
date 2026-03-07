import { defineConfig } from "vite";

export default defineConfig({
	build: {
		outDir: "build",
		assetsInlineLimit: 100_000,
	},
});
