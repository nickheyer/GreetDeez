import type { DoomScene } from "../scene/DoomScene.js";
import type { TerminalScreen } from "../ui3d/TerminalScreen.js";

const WALK_DURATION = 5.5;
const WALK_TARGET_Z = -24;

/**
 * Boot sequence:
 * 1. Fade in from black (1s)
 * 2. Camera walks corridor Z=1→-24 (7s, ease-in-out), lights activate as camera passes,
 *    keyboard weapon bobs with walk cycle
 * 3. Terminal screen boots: typing animation with UAC messages
 * 4. Form fields appear, keyboard input enabled
 */
export async function runBootSequence(scene: DoomScene, terminal: TerminalScreen) {
	scene.opacity = 0;
	scene.bloomStrength = 0.3;

	// Phase 1: Fade in (1s)
	await fadeIn(scene, 1000);

	// Phase 2: Walk down corridor (7s)
	scene.weapon.setWalkBob(true);
	const walkPromise = scene.dollyTo(WALK_TARGET_Z, WALK_DURATION);

	// Activate lights as camera passes during walk
	const lightsPromise = activateLightsDuringWalk(scene);

	await Promise.all([walkPromise, lightsPromise]);
	scene.weapon.setWalkBob(false);
	scene.lights.enableAll();

	// Reduce camera sway after boot completes
	scene.swayAmplitude = 0.3;

	// Phase 3+4: Terminal boot + form reveal
	scene.bloomStrength = 1.5;
	await terminal.runBootSequence();
}

function fadeIn(scene: DoomScene, durationMs: number): Promise<void> {
	return new Promise((resolve) => {
		const start = performance.now();
		const step = (now: number) => {
			const t = Math.min((now - start) / durationMs, 1);
			scene.opacity = t;
			scene.bloomStrength = 0.3 + t * 1.2;

			if (t < 1) {
				requestAnimationFrame(step);
			} else {
				resolve();
			}
		};
		requestAnimationFrame(step);
	});
}

function activateLightsDuringWalk(scene: DoomScene): Promise<void> {
	return new Promise((resolve) => {
		const start = performance.now();
		const duration = WALK_DURATION * 1000;
		const step = (now: number) => {
			const t = Math.min((now - start) / duration, 1);
			const cameraZ = 1 + (WALK_TARGET_Z - 1) * t;
			scene.lights.enableUpTo(cameraZ);

			if (t < 1) {
				requestAnimationFrame(step);
			} else {
				resolve();
			}
		};
		requestAnimationFrame(step);
	});
}
