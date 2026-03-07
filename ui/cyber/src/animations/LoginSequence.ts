import type { CyberScene } from "../scene/CyberScene.js";

/** Glitch burst -> camera dolly -> bloom whiteout */
export async function runLoginSequence(scene: CyberScene): Promise<void> {
	// Glitch burst
	scene.glitchEnabled = true;
	await sleep(300);
	scene.glitchEnabled = false;

	// Camera dolly forward + bloom surge
	const dolly = scene.dollyForward(1.2);
	const bloom = surgeBloom(scene, 1.2);
	await Promise.all([dolly, bloom]);

	// Hold white
	await sleep(500);
}

function surgeBloom(scene: CyberScene, durationS: number): Promise<void> {
	return new Promise((resolve) => {
		const start = performance.now();
		const durationMs = durationS * 1000;
		const step = (now: number) => {
			const t = Math.min((now - start) / durationMs, 1);
			scene.bloomStrength = 1.2 + t * 6;
			scene.opacity = 1 + t * 0.5; // >1 goes very bright

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
