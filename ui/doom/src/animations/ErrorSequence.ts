import type { DoomScene } from "../scene/DoomScene.js";

let activeAbort: AbortController | null = null;

/**
 * Auth failure:
 * 1. Glitch burst 300ms
 * 2. Bloom spike to 3.0 (500ms)
 * 3. All lights go red (500ms)
 *
 * Uses AbortController to cancel previous error sequence on rapid failures.
 */
export async function runErrorSequence(scene: DoomScene): Promise<void> {
	// Cancel any in-flight error sequence
	if (activeAbort) {
		activeAbort.abort();
	}
	const abort = new AbortController();
	activeAbort = abort;
	const signal = abort.signal;

	const origBloom = scene.bloomStrength;

	// Glitch burst
	scene.glitchEnabled = true;

	// Bloom spike + lights red
	scene.bloomStrength = 3.0;
	scene.lightsRedOverride = true;

	await sleepAbortable(300, signal);
	if (signal.aborted) return;

	scene.glitchEnabled = false;

	await sleepAbortable(200, signal);
	if (signal.aborted) return;

	// Restore
	scene.bloomStrength = origBloom;
	scene.lightsRedOverride = false;

	if (activeAbort === abort) {
		activeAbort = null;
	}
}

function sleepAbortable(ms: number, signal: AbortSignal): Promise<void> {
	return new Promise((resolve) => {
		if (signal.aborted) { resolve(); return; }
		const id = setTimeout(resolve, ms);
		signal.addEventListener("abort", () => {
			clearTimeout(id);
			resolve();
		}, { once: true });
	});
}
