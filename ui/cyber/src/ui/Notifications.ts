export class Notifications {
	private errorEl: HTMLElement;
	private capsEl: HTMLElement;
	private typeTimer: number | null = null;

	constructor(container: HTMLElement) {
		this.errorEl = document.createElement("div");
		this.errorEl.className = "cyber-error";
		container.appendChild(this.errorEl);

		this.capsEl = document.createElement("div");
		this.capsEl.className = "cyber-caps";
		container.appendChild(this.capsEl);
	}

	showError(msg: string) {
		if (this.typeTimer !== null) {
			clearInterval(this.typeTimer);
			this.typeTimer = null;
		}

		this.errorEl.textContent = "";
		let i = 0;
		this.typeTimer = window.setInterval(() => {
			if (i < msg.length) {
				this.errorEl.textContent += msg[i];
				i++;
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
		this.errorEl.textContent = "";
	}

	setCapsLock(on: boolean) {
		this.capsEl.textContent = on ? "CAPS LOCK ACTIVE" : "";
	}
}
