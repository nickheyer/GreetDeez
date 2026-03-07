import * as THREE from "three";
import { createText } from "./text-helper.js";

export class Notifications3D extends THREE.Group {
	private errorText: ReturnType<typeof createText>;
	private capsText: ReturnType<typeof createText>;
	private typeTimer: number | null = null;

	constructor() {
		super();

		this.errorText = createText({
			text: "",
			fontSize: 0.05,
			color: 0xff0044,
			anchorX: "center",
		});
		this.errorText.position.set(0, 0, 0);
		this.add(this.errorText);

		this.capsText = createText({
			text: "",
			fontSize: 0.04,
			color: 0xffaa00,
			anchorX: "center",
		});
		this.capsText.position.set(0, -0.08, 0);
		this.add(this.capsText);
	}

	showError(msg: string) {
		this.clearError();
		let i = 0;
		this.typeTimer = window.setInterval(() => {
			if (i < msg.length) {
				i++;
				this.errorText.text = msg.slice(0, i);
				this.errorText.sync();
			} else {
				clearInterval(this.typeTimer!);
				this.typeTimer = null;
			}
		}, 25);
	}

	clearError() {
		if (this.typeTimer !== null) {
			clearInterval(this.typeTimer);
			this.typeTimer = null;
		}
		this.errorText.text = "";
		this.errorText.sync();
	}

	setCapsLock(on: boolean) {
		this.capsText.text = on ? "CAPS LOCK ACTIVE" : "";
		this.capsText.sync();
	}
}
