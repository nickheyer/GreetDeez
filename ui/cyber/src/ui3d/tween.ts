export function sleep(ms: number): Promise<void> {
	return new Promise((r) => setTimeout(r, ms));
}

export function tweenValue(
	from: number,
	to: number,
	durationMs: number,
	onUpdate: (v: number) => void,
	easing: (t: number) => number = easeOutCubic,
): Promise<void> {
	return new Promise((resolve) => {
		const start = performance.now();
		const step = (now: number) => {
			const raw = Math.min((now - start) / durationMs, 1);
			const t = easing(raw);
			onUpdate(from + (to - from) * t);
			if (raw < 1) {
				requestAnimationFrame(step);
			} else {
				resolve();
			}
		};
		requestAnimationFrame(step);
	});
}

export function easeOutCubic(t: number): number {
	return 1 - (1 - t) ** 3;
}

export function easeInOutQuad(t: number): number {
	return t < 0.5 ? 2 * t * t : 1 - (-2 * t + 2) ** 2 / 2;
}

export function linear(t: number): number {
	return t;
}
