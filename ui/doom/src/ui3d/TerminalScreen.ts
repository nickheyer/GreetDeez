import * as THREE from "three";
import { createText } from "./text-helper.js";
import { getClient } from "../client.js";
import { sleep } from "./tween.js";

interface Session {
	name: string;
	cmd: string[];
	type: number;
	desktop: string;
}

export interface TerminalScreenCallbacks {
	onSuccess: () => void;
	onError: (msg: string) => void;
	onBoot: () => void;
}

type TerminalState = "boot" | "idle" | "session-select" | "authenticating" | "success" | "error";
type IdleField = "user" | "pass";

const SCREEN_W = 1.2;
const SCREEN_H = 1.1;
const LINE_COUNT = 16;
const CHAR_LIMIT = 22;
const CURSOR_BLINK_MS = 500;

const COLOR_AMBER = 0xffaa00;
const COLOR_TEXT = 0xffddaa;
const COLOR_MUTED = 0x997733;
const COLOR_RED = 0xff4422;
const COLOR_GREEN = 0x44ff44;

export class TerminalScreen extends THREE.Group {
	private lines: ReturnType<typeof createText>[] = [];
	private state: TerminalState = "boot";
	private idleField: IdleField = "user";
	private callbacks: TerminalScreenCallbacks;

	private username = "";
	private password = "";
	private sessions: Session[] = [];
	private selectedSession = 0;
	private hostname = "";

	private cursorVisible = true;
	private cursorBlinkId = 0;
	private clockTimerId = 0;
	private capsLockOn = false;
	private busy = false;
	private errorAbort: AbortController | null = null;

	private keyHandler: ((e: KeyboardEvent) => void) | null = null;

	constructor(callbacks: TerminalScreenCallbacks) {
		super();
		this.callbacks = callbacks;

		const lineH = SCREEN_H / LINE_COUNT;
		const startY = SCREEN_H / 2 - lineH * 0.5;
		const leftX = -SCREEN_W / 2 + 0.04;

		for (let i = 0; i < LINE_COUNT; i++) {
			const t = createText({
				text: "",
				fontSize: 0.048,
				color: COLOR_TEXT,
				anchorX: "left",
				anchorY: "middle",
				letterSpacing: 0.015,
			});
			// Text sits just barely in front of the CRT screen surface
			t.position.set(leftX, startY - i * lineH, 0.003);
			this.add(t);
			this.lines.push(t);
		}

		this.visible = false;
	}

	// --- Public API ---

	setSessions(sessions: Session[]) {
		this.sessions = sessions;
		if (this.state === "idle" || this.state === "error") this.renderIdle();
	}

	setSelectedSession(index: number) {
		this.selectedSession = index;
		if (this.state === "idle" || this.state === "error") this.renderIdle();
	}

	setHostname(name: string) {
		this.hostname = name;
	}

	setUsername(name: string) {
		this.username = name;
	}

	showError(msg: string) {
		if (this.state !== "idle" && this.state !== "error") return;
		this.cancelError();
		this.state = "error";
		this.renderError(msg);
	}

	update(_time: number) {
		// Reserved for per-frame effects
	}

	// --- Boot Sequence ---

	async runBootSequence(): Promise<void> {
		this.visible = true;
		this.state = "boot";
		this.clearAll();

		const bootLines = [
			"UAC SYSTEMS v6.66",
			"MARS FACILITY // SECTOR 7-G",
			"================================",
		];

		for (let i = 0; i < bootLines.length; i++) {
			await this.typeLine(i, bootLines[i], COLOR_AMBER);
			await sleep(300);
		}

		if (this.hostname) {
			await this.typeLine(3, this.hostname, COLOR_MUTED);
			await sleep(200);
		}

		this.callbacks.onBoot();

		// Transition to idle
		this.state = "idle";
		this.idleField = "user";
		this.renderIdle();
		this.startCursorBlink();
		this.startClock();
		this.attachKeyboard();
	}

	// --- Rendering ---

	private clearAll() {
		for (const line of this.lines) {
			line.text = "";
			line.color = COLOR_TEXT;
			line.sync();
		}
	}

	private setLine(index: number, text: string, color: number = COLOR_TEXT) {
		if (index < 0 || index >= LINE_COUNT) return;
		const line = this.lines[index];
		line.text = text;
		line.color = color;
		line.sync();
	}

	private async typeLine(index: number, text: string, color: number = COLOR_TEXT): Promise<void> {
		if (index < 0 || index >= LINE_COUNT) return;
		const line = this.lines[index];
		line.color = color;
		for (let i = 0; i <= text.length; i++) {
			line.text = text.slice(0, i);
			line.sync();
			await sleep(25 + Math.random() * 15);
		}
	}

	private renderIdle() {
		// Layout:
		//  0: UAC SYSTEMS v6.66
		//  1: MARS FACILITY // SECTOR 7-G
		//  2: ================================
		//  3: hostname
		//  4: (blank)
		//  5: IDENTIFY: username_
		//  6: ACCESS CODE: ********
		//  7: (blank)
		//  8: MISSION: [SessionName] (</>)
		//  9: (blank)
		// 10: > ENGAGE [ENTER]
		// 11: (blank)
		// 12: error / status line
		// 13: caps lock warning
		// 14: (blank)
		// 15: clock

		const cursor = this.cursorVisible ? "_" : " ";

		// Username
		if (this.idleField === "user") {
			this.setLine(5, "IDENTIFY: " + this.username + cursor, COLOR_TEXT);
		} else {
			this.setLine(5, "IDENTIFY: " + this.username, COLOR_MUTED);
		}

		// Password
		const passDisplay = "*".repeat(this.password.length);
		if (this.idleField === "pass") {
			this.setLine(6, "ACCESS CODE: " + passDisplay + cursor, COLOR_TEXT);
		} else {
			this.setLine(6, "ACCESS CODE: " + passDisplay, COLOR_MUTED);
		}

		this.setLine(4, "");
		this.setLine(7, "");

		// Session
		if (this.sessions.length > 0) {
			const sName = this.sessions[this.selectedSession]?.name ?? "---";
			const arrows = this.sessions.length > 1 ? " (</>)" : "";
			this.setLine(8, "MISSION: [" + sName + "]" + arrows, COLOR_AMBER);
		} else {
			this.setLine(8, "MISSION: [---]", COLOR_MUTED);
		}

		this.setLine(9, "");

		// Submit
		if (this.busy) {
			this.setLine(10, "> AUTHENTICATING...", COLOR_AMBER);
		} else {
			this.setLine(10, "> ENGAGE [ENTER]", COLOR_AMBER);
		}

		this.setLine(11, "");

		// Caps lock
		this.setLine(13, this.capsLockOn ? "WARNING: CAPS LOCK ENGAGED" : "", COLOR_AMBER);
		this.setLine(14, "");
	}

	private renderError(msg: string) {
		this.renderIdle();
		this.setLine(12, "! " + msg, COLOR_RED);

		const abort = new AbortController();
		this.errorAbort = abort;
		setTimeout(() => {
			if (abort.signal.aborted) return;
			this.state = "idle";
			this.setLine(12, "");
			this.errorAbort = null;
		}, 3000);
	}

	private cancelError() {
		if (this.errorAbort) {
			this.errorAbort.abort();
			this.errorAbort = null;
		}
		this.setLine(12, "");
	}

	// --- Clock (bottom line) ---

	private startClock() {
		this.tickClock();
		this.clockTimerId = window.setInterval(() => this.tickClock(), 1000);
	}

	private tickClock() {
		const now = new Date();
		const h = String(now.getHours()).padStart(2, "0");
		const m = String(now.getMinutes()).padStart(2, "0");
		const s = String(now.getSeconds()).padStart(2, "0");
		this.setLine(15, `UAC TIMEBASE: ${h}:${m}:${s}`, COLOR_MUTED);
	}

	// --- Cursor ---

	private startCursorBlink() {
		this.stopCursorBlink();
		this.cursorVisible = true;
		this.cursorBlinkId = window.setInterval(() => {
			this.cursorVisible = !this.cursorVisible;
			if (this.state === "idle" || this.state === "error") {
				this.renderIdle();
			}
		}, CURSOR_BLINK_MS);
	}

	private stopCursorBlink() {
		if (this.cursorBlinkId) {
			clearInterval(this.cursorBlinkId);
			this.cursorBlinkId = 0;
		}
		this.cursorVisible = false;
	}

	// --- Keyboard ---

	private attachKeyboard() {
		this.detachKeyboard();
		this.keyHandler = (e: KeyboardEvent) => this.handleKey(e);
		document.addEventListener("keydown", this.keyHandler);
	}

	private detachKeyboard() {
		if (this.keyHandler) {
			document.removeEventListener("keydown", this.keyHandler);
			this.keyHandler = null;
		}
	}

	private handleKey(e: KeyboardEvent) {
		if (this.state !== "idle" && this.state !== "error") return;
		if (this.busy) return;

		const caps = e.getModifierState("CapsLock");
		if (caps !== this.capsLockOn) {
			this.capsLockOn = caps;
			this.renderIdle();
		}

		// Arrow keys cycle sessions
		if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
			e.preventDefault();
			if (this.sessions.length > 1) {
				if (e.key === "ArrowLeft") {
					this.selectedSession = (this.selectedSession - 1 + this.sessions.length) % this.sessions.length;
				} else {
					this.selectedSession = (this.selectedSession + 1) % this.sessions.length;
				}
				this.renderIdle();
			}
			return;
		}

		// Tab switches fields
		if (e.key === "Tab") {
			e.preventDefault();
			this.idleField = this.idleField === "user" ? "pass" : "user";
			this.cursorVisible = true;
			this.renderIdle();
			return;
		}

		// Enter submits
		if (e.key === "Enter") {
			e.preventDefault();
			if (!this.username) {
				this.showError("IDENTIFY REQUIRED");
				this.idleField = "user";
				return;
			}
			if (!this.sessions.length) {
				this.showError("NO MISSIONS AVAILABLE");
				return;
			}
			this.handleLogin();
			return;
		}

		// Backspace
		if (e.key === "Backspace") {
			e.preventDefault();
			if (this.idleField === "user" && this.username.length > 0) {
				this.username = this.username.slice(0, -1);
			} else if (this.idleField === "pass" && this.password.length > 0) {
				this.password = this.password.slice(0, -1);
			}
			this.cancelError();
			this.state = "idle";
			this.renderIdle();
			return;
		}

		// Ctrl+A: clear field
		if ((e.ctrlKey || e.metaKey) && e.key === "a") {
			e.preventDefault();
			if (this.idleField === "user") this.username = "";
			else this.password = "";
			this.renderIdle();
			return;
		}

		// Ignore modifier combos
		if (e.ctrlKey || e.altKey || e.metaKey) return;
		if (e.key.length !== 1) return;

		// Type character
		e.preventDefault();
		if (this.idleField === "user") {
			if (this.username.length < CHAR_LIMIT) {
				this.username += e.key;
			}
		} else {
			if (this.password.length < CHAR_LIMIT) {
				this.password += e.key;
			}
		}
		this.cancelError();
		this.state = "idle";
		this.cursorVisible = true;
		this.renderIdle();
	}

	// --- Auth Flow ---

	private async handleLogin() {
		this.cancelError();
		this.busy = true;
		this.stopCursorBlink();
		this.renderIdle();

		try {
			const session = this.sessions[this.selectedSession];
			const client = getClient();

			const authResp = await client.authenticate({
				username: this.username,
				password: this.password,
			});

			if (!authResp.success) {
				this.onAuthError(authResp.error || "ACCESS DENIED");
				return;
			}

			const startResp = await client.startSession({ session });
			if (!startResp.success) {
				this.onAuthError(startResp.error || "SESSION FAILED");
				return;
			}

			await client.saveState({
				state: {
					lastUser: this.username,
					lastSession: session.name,
				},
			});

			this.state = "success";
			this.detachKeyboard();
			this.setLine(10, "> ACCESS GRANTED", COLOR_GREEN);
			this.setLine(12, "");
			this.callbacks.onSuccess();
		} catch (e) {
			this.onAuthError(String(e));
		}
	}

	private onAuthError(msg: string) {
		this.busy = false;
		this.password = "";
		this.idleField = "pass";
		this.state = "error";
		this.startCursorBlink();
		this.renderError(msg);
		this.callbacks.onError(msg);
	}

	// --- Disposal ---

	dispose() {
		this.detachKeyboard();
		this.stopCursorBlink();
		if (this.clockTimerId) clearInterval(this.clockTimerId);
		this.cancelError();
		for (const line of this.lines) {
			line.geometry.dispose();
			(line.material as THREE.Material).dispose();
		}
	}
}
