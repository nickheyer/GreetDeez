import type { CyberScene } from "../scene/CyberScene.js";

/** Glitch burst + brief red-tinted bloom spike on auth failure */
export function runErrorSequence(scene: CyberScene) {
	// Glitch burst
	scene.glitchEnabled = true;
	setTimeout(() => {
		scene.glitchEnabled = false;
	}, 250);

	// Brief bloom spike
	const origBloom = scene.bloomStrength;
	scene.bloomStrength = 2.5;
	setTimeout(() => {
		scene.bloomStrength = origBloom;
	}, 300);
}
