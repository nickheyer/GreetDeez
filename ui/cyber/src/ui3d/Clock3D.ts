import * as THREE from "three";
import { createText } from "./text-helper.js";

export class Clock3D extends THREE.Group {
	private clockText: ReturnType<typeof createText>;
	private interval: number;
	private showColon = true;

	constructor() {
		super();

		this.clockText = createText({
			text: "",
			fontSize: 0.1,
			color: 0x00ffff,
			anchorX: "right",
			anchorY: "top",
			letterSpacing: 0.03,
		});
		this.clockText.material.opacity = 0.8;
		this.add(this.clockText);

		this.tick();
		this.interval = window.setInterval(() => this.tick(), 500);
	}

	private tick() {
		const now = new Date();
		const h = String(now.getHours()).padStart(2, "0");
		const m = String(now.getMinutes()).padStart(2, "0");
		const s = String(now.getSeconds()).padStart(2, "0");
		const sep = this.showColon ? ":" : " ";
		this.showColon = !this.showColon;
		this.clockText.text = `${h}${sep}${m}${sep}${s}`;
		this.clockText.sync();
	}

	dispose() {
		clearInterval(this.interval);
	}
}
