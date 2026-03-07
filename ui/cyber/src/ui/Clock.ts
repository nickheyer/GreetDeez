export class Clock {
	private el: HTMLElement;
	private interval: number;

	constructor(parent: HTMLElement) {
		this.el = document.createElement("div");
		this.el.className = "cyber-clock";
		parent.appendChild(this.el);

		this.tick();
		this.interval = window.setInterval(() => this.tick(), 1000);
	}

	private tick() {
		const now = new Date();
		const h = String(now.getHours()).padStart(2, "0");
		const m = String(now.getMinutes()).padStart(2, "0");
		const s = String(now.getSeconds()).padStart(2, "0");
		this.el.innerHTML = `${h}<span class="colon">:</span>${m}<span class="colon">:</span>${s}`;
	}

	dispose() {
		clearInterval(this.interval);
		this.el.remove();
	}
}
