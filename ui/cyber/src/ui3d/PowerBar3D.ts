import * as THREE from "three";
import { Button3D } from "./Button3D.js";
import type { SceneInteraction } from "./SceneInteraction.js";
import { getClient } from "../client.js";

interface PowerCaps {
	canPoweroff: boolean;
	canReboot: boolean;
	canSuspend: boolean;
}

export class PowerBar3D extends THREE.Group {
	private interaction: SceneInteraction;
	private onError: (msg: string) => void;
	private buttons: Button3D[] = [];

	constructor(interaction: SceneInteraction, onError: (msg: string) => void) {
		super();
		this.interaction = interaction;
		this.onError = onError;
	}

	setCaps(caps: PowerCaps) {
		// Clear existing
		this.buttons.forEach((b) => this.remove(b));
		this.buttons = [];

		const entries: { label: string; action: number }[] = [];
		if (caps.canPoweroff) entries.push({ label: "[PWR]", action: 1 });
		if (caps.canReboot) entries.push({ label: "[RST]", action: 2 });
		if (caps.canSuspend) entries.push({ label: "[SLP]", action: 3 });

		entries.forEach((entry, i) => {
			const btn = new Button3D({
				label: entry.label,
				width: 0.35,
				height: 0.15,
				fontSize: 0.04,
				onClick: async () => {
					try {
						await getClient().executePowerAction({ action: entry.action });
					} catch (e) {
						this.onError(String(e));
					}
				},
				interaction: this.interaction,
			});
			btn.position.x = i * 0.45;
			this.add(btn);
			this.buttons.push(btn);
		});
	}
}
