import { getClient } from "../client.js";

interface PowerCaps {
	canPoweroff: boolean;
	canReboot: boolean;
	canSuspend: boolean;
}

export class PowerBar {
	private el: HTMLElement;
	private onError: (msg: string) => void;

	constructor(parent: HTMLElement, onError: (msg: string) => void) {
		this.el = document.createElement("div");
		this.el.className = "cyber-power";
		this.onError = onError;
		parent.appendChild(this.el);
	}

	setCaps(caps: PowerCaps) {
		this.el.innerHTML = "";

		if (caps.canPoweroff) {
			this.addButton("[PWR]", 1);
		}
		if (caps.canReboot) {
			this.addButton("[RST]", 2);
		}
		if (caps.canSuspend) {
			this.addButton("[SLP]", 3);
		}
	}

	private addButton(label: string, action: number) {
		const btn = document.createElement("button");
		btn.textContent = label;
		btn.addEventListener("click", async () => {
			try {
				await getClient().executePowerAction({ action });
			} catch (e) {
				this.onError(String(e));
			}
		});
		this.el.appendChild(btn);
	}
}
