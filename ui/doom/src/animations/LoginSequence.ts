import type { DoomScene } from "../scene/DoomScene.js";

/**
 * Login success:
 * 1. Bloom spike to 4.0 (amber flash)
 * 2. Camera dolly Z=-24→-30 (1s) + bloom surge to 8+ (parallel)
 * 3. Hold bright 500ms
 */
export async function runLoginSequence(scene: DoomScene): Promise<void> {
	// Bloom spike
	scene.bloomStrength = 4.0;
	await sleep(200);

	// Camera dolly + bloom surge
	const dolly = scene.dollyTo(-30, 1.0);
	const bloom = surgeBloom(scene, 1.0);
	await Promise.all([dolly, bloom]);

	// Hold bright
	await sleep(500);
}

function surgeBloom(scene: DoomScene, durationS: number): Promise<void> {
	return new Promise((resolve) => {
		const start = performance.now();
		const durationMs = durationS * 1000;
		const step = (now: number) => {
			const t = Math.min((now - start) / durationMs, 1);
			scene.bloomStrength = 4.0 + t * 6;
			scene.opacity = Math.min(1.0 + t * 0.5, 1.0);

			if (t < 1) {
				requestAnimationFrame(step);
			} else {
				resolve();
			}
		};
		requestAnimationFrame(step);
	});
}

function sleep(ms: number): Promise<void> {
	return new Promise((r) => setTimeout(r, ms));
}
