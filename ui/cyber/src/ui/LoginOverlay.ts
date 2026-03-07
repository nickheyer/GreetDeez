import { getClient } from "../client.js";
import { Notifications } from "./Notifications.js";

interface Session {
	name: string;
	cmd: string[];
	type: number;
	desktop: string;
}

export interface LoginCallbacks {
	onSuccess: () => void;
	onError: (msg: string) => void;
	onBoot: () => void;
}

export class LoginOverlay {
	private panel: HTMLElement;
	private bottomCorners: HTMLElement;
	private bootText: HTMLElement;
	private hostnameEl: HTMLElement;
	private userField: HTMLElement;
	private userInput: HTMLInputElement;
	private passField: HTMLElement;
	private passInput: HTMLInputElement;
	private sessionField: HTMLElement;
	private sessionTrigger: HTMLButtonElement;
	private sessionMenu: HTMLElement | null = null;
	private submitBtn: HTMLButtonElement;
	private notifications: Notifications;

	private sessions: Session[] = [];
	private selectedSession = 0;
	private busy = false;
	private callbacks: LoginCallbacks;

	constructor(parent: HTMLElement, callbacks: LoginCallbacks) {
		this.callbacks = callbacks;

		this.panel = document.createElement("div");
		this.panel.className = "cyber-panel";
		parent.appendChild(this.panel);

		this.bottomCorners = document.createElement("div");
		this.bottomCorners.className = "cyber-panel-bottom";
		this.panel.appendChild(this.bottomCorners);

		// Boot text
		this.bootText = document.createElement("div");
		this.bootText.className = "boot-text";
		this.panel.appendChild(this.bootText);

		// Hostname
		this.hostnameEl = document.createElement("div");
		this.hostnameEl.className = "cyber-hostname";
		this.hostnameEl.style.display = "none";
		this.panel.appendChild(this.hostnameEl);

		// Username field
		this.userField = document.createElement("div");
		this.userField.className = "cyber-field";
		this.userField.innerHTML = `<label>Username</label>`;
		this.userInput = document.createElement("input");
		this.userInput.type = "text";
		this.userInput.autocomplete = "off";
		this.userInput.spellcheck = false;
		this.userInput.addEventListener("keydown", (e) => this.handleKeyDown(e));
		this.userField.appendChild(this.userInput);
		this.panel.appendChild(this.userField);

		// Password field
		this.passField = document.createElement("div");
		this.passField.className = "cyber-field";
		this.passField.innerHTML = `<label>Password</label>`;
		this.passInput = document.createElement("input");
		this.passInput.type = "password";
		this.passInput.addEventListener("keydown", (e) => this.handleKeyDown(e));
		this.passField.appendChild(this.passInput);
		this.panel.appendChild(this.passField);

		// Session picker (hidden until multiple sessions)
		this.sessionField = document.createElement("div");
		this.sessionField.className = "cyber-session";
		this.sessionField.innerHTML = `<label>Session</label>`;
		this.sessionField.style.display = "none";
		this.sessionTrigger = document.createElement("button");
		this.sessionTrigger.className = "cyber-session-trigger";
		this.sessionTrigger.type = "button";
		this.sessionTrigger.textContent = "---";
		this.sessionTrigger.addEventListener("click", () => this.toggleSessionMenu());
		this.sessionField.appendChild(this.sessionTrigger);
		this.panel.appendChild(this.sessionField);

		// Submit
		this.submitBtn = document.createElement("button");
		this.submitBtn.className = "cyber-submit";
		this.submitBtn.type = "button";
		this.submitBtn.textContent = "ACCESS SYSTEM";
		this.submitBtn.addEventListener("click", () => this.handleLogin());
		this.panel.appendChild(this.submitBtn);

		// Notifications
		this.notifications = new Notifications(this.panel);
	}

	private handleKeyDown(e: KeyboardEvent) {
		this.notifications.setCapsLock(e.getModifierState("CapsLock"));

		if (e.key === "Enter") {
			if (e.target === this.userInput) {
				this.passInput.focus();
			} else {
				this.handleLogin();
			}
		}
	}

	private toggleSessionMenu() {
		if (this.sessionMenu) {
			this.closeSessionMenu();
			return;
		}

		const backdrop = document.createElement("div");
		backdrop.className = "cyber-session-backdrop";
		backdrop.addEventListener("click", () => this.closeSessionMenu());
		this.sessionField.appendChild(backdrop);

		this.sessionMenu = document.createElement("div");
		this.sessionMenu.className = "cyber-session-menu";

		this.sessions.forEach((s, i) => {
			const item = document.createElement("button");
			item.className = "cyber-session-item" + (i === this.selectedSession ? " active" : "");
			item.type = "button";
			item.textContent = s.name;
			item.addEventListener("click", () => {
				this.selectedSession = i;
				this.sessionTrigger.textContent = s.name;
				this.closeSessionMenu();
			});
			this.sessionMenu!.appendChild(item);
		});

		this.sessionField.appendChild(this.sessionMenu);
	}

	private closeSessionMenu() {
		if (this.sessionMenu) {
			this.sessionMenu.remove();
			this.sessionMenu = null;
		}
		const backdrop = this.sessionField.querySelector(".cyber-session-backdrop");
		backdrop?.remove();
	}

	private async handleLogin() {
		if (this.busy || !this.userInput.value || !this.sessions.length) return;
		this.notifications.clearError();
		this.busy = true;
		this.submitBtn.disabled = true;
		this.submitBtn.textContent = "AUTHENTICATING...";

		try {
			const session = this.sessions[this.selectedSession];
			const client = getClient();

			const authResp = await client.authenticate({
				username: this.userInput.value,
				password: this.passInput.value,
			});

			if (!authResp.success) {
				this.onAuthError(authResp.error || "Authentication failed");
				return;
			}

			const startResp = await client.startSession({ session });
			if (!startResp.success) {
				this.onAuthError(startResp.error || "Session start failed");
				return;
			}

			await client.saveState({
				state: {
					lastUser: this.userInput.value,
					lastSession: session.name,
				},
			});

			this.callbacks.onSuccess();
		} catch (e) {
			this.onAuthError(String(e));
		}
	}

	private onAuthError(msg: string) {
		this.busy = false;
		this.submitBtn.disabled = false;
		this.submitBtn.textContent = "ACCESS SYSTEM";
		this.passInput.value = "";
		this.passInput.focus();

		this.notifications.showError(msg);
		this.callbacks.onError(msg);

		// Shake + red flash
		this.panel.classList.add("shake", "error-flash");
		setTimeout(() => {
			this.panel.classList.remove("shake", "error-flash");
		}, 600);
	}

	setSessions(sessions: Session[]) {
		this.sessions = sessions;
		if (sessions.length > 1) {
			this.sessionField.style.display = "";
			this.sessionTrigger.textContent = sessions[this.selectedSession]?.name ?? "---";
		}
	}

	setSelectedSession(index: number) {
		this.selectedSession = index;
		if (this.sessions.length > 1) {
			this.sessionTrigger.textContent = this.sessions[index]?.name ?? "---";
		}
	}

	setHostname(name: string) {
		if (name) {
			this.hostnameEl.textContent = name;
			this.hostnameEl.style.display = "";
		}
	}

	setUsername(name: string) {
		this.userInput.value = name;
	}

	/** Run the boot typing sequence, then reveal form fields */
	async runBootSequence(): Promise<void> {
		this.panel.classList.add("visible");

		const lines = [
			"SYSTEM BOOT...",
			"ESTABLISHING CONNECTION...",
			"ACCESS TERMINAL READY",
		];

		for (const line of lines) {
			await this.typeText(line);
			await sleep(400);
		}

		this.callbacks.onBoot();

		this.bootText.textContent = "";

		// Staggered reveal of form elements
		const fields = [this.userField, this.passField];
		if (this.sessions.length > 1) fields.push(this.sessionField);

		for (let i = 0; i < fields.length; i++) {
			await sleep(100);
			fields[i].classList.add("visible");
		}

		await sleep(100);
		this.submitBtn.classList.add("visible");
		this.userInput.focus();
	}

	private async typeText(text: string): Promise<void> {
		this.bootText.textContent = "";
		for (let i = 0; i < text.length; i++) {
			this.bootText.textContent = text.slice(0, i + 1);
			await sleep(30 + Math.random() * 20);
		}
	}
}

function sleep(ms: number): Promise<void> {
	return new Promise((r) => setTimeout(r, ms));
}
