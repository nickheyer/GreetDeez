import type { CyberScene } from "../scene/CyberScene.js";
import type { LoginOverlay } from "../ui/LoginOverlay.js";

/** Fades in the 3D scene while the login overlay runs its boot typing */
export async function runBootSequence(scene: CyberScene, overlay: LoginOverlay) {
	// Start with scene invisible
	scene.opacity = 0;
	scene.bloomStrength = 0.3;

	// Fade in scene over ~2s in parallel with overlay boot text
	const fadePromise = fadeIn(scene, 2000);
	const bootPromise = overlay.runBootSequence();

	await Promise.all([fadePromise, bootPromise]);

	// Final bloom ramp
	scene.bloomStrength = 1.2;
}

function fadeIn(scene: CyberScene, durationMs: number): Promise<void> {
	return new Promise((resolve) => {
		const start = performance.now();
		const step = (now: number) => {
			const t = Math.min((now - start) / durationMs, 1);
			scene.opacity = t;
			scene.bloomStrength = 0.3 + t * 0.9;

			if (t < 1) {
				requestAnimationFrame(step);
			} else {
				resolve();
			}
		};
		requestAnimationFrame(step);
	});
}
