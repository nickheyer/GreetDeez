import * as THREE from "three";
import { createText } from "./text-helper.js";
import { PanelMaterial } from "./materials/PanelMaterial.js";
import { TextInput3D } from "./TextInput3D.js";
import { Button3D } from "./Button3D.js";
import { SessionPicker3D } from "./SessionPicker3D.js";
import { Notifications3D } from "./Notifications3D.js";
import { getClient } from "../client.js";
import { sleep, tweenValue, linear } from "./tween.js";
import type { SceneInteraction } from "./SceneInteraction.js";

interface Session {
	name: string;
	cmd: string[];
	type: number;
	desktop: string;
}

export interface LoginPanel3DCallbacks {
	onSuccess: () => void;
	onError: (msg: string) => void;
	onBoot: () => void;
}

const PANEL_W = 2.4;
const PANEL_H = 3.2;

export class LoginPanel3D extends THREE.Group {
	private panelMat: PanelMaterial;
	private bootText: ReturnType<typeof createText>;
	private hostnameText: ReturnType<typeof createText>;
	private userInput: TextInput3D;
	private passInput: TextInput3D;
	private sessionPicker: SessionPicker3D;
	private submitBtn: Button3D;
	private notifications: Notifications3D;
	private interaction: SceneInteraction;

	private sessions: Session[] = [];
	private busy = false;
	private callbacks: LoginPanel3DCallbacks;

	// Groups for stagger animation
	private userGroup: THREE.Group;
	private passGroup: THREE.Group;
	private sessionGroup: THREE.Group;
	private submitGroup: THREE.Group;

	constructor(interaction: SceneInteraction, callbacks: LoginPanel3DCallbacks) {
		super();
		this.interaction = interaction;
		this.callbacks = callbacks;

		// Background plane
		this.panelMat = new PanelMaterial(PANEL_W, PANEL_H);
		const bgPlane = new THREE.Mesh(
			new THREE.PlaneGeometry(PANEL_W, PANEL_H),
			this.panelMat,
		);
		this.add(bgPlane);

		// Border lines
		const hw = PANEL_W / 2;
		const hh = PANEL_H / 2;
		const borderPts = [
			-hw, -hh, 0, hw, -hh, 0,
			hw, -hh, 0, hw, hh, 0,
			hw, hh, 0, -hw, hh, 0,
			-hw, hh, 0, -hw, -hh, 0,
		];
		const borderGeo = new THREE.BufferGeometry();
		borderGeo.setAttribute("position", new THREE.Float32BufferAttribute(borderPts, 3));
		const borderMat = new THREE.LineBasicMaterial({
			color: 0x00ffff,
			transparent: true,
			opacity: 0.3,
			blending: THREE.AdditiveBlending,
		});
		const border = new THREE.LineSegments(borderGeo, borderMat);
		border.position.z = 0.001;
		this.add(border);

		// Corner brackets
		this.addCornerBrackets(hw, hh);

		// Boot text (centered)
		this.bootText = createText({
			text: "",
			fontSize: 0.055,
			color: 0x00ffff,
			anchorX: "center",
			letterSpacing: 0.04,
		});
		this.bootText.position.set(0, 1.0, 0.01);
		this.add(this.bootText);

		// Hostname
		this.hostnameText = createText({
			text: "",
			fontSize: 0.08,
			color: 0x00ffff,
			anchorX: "center",
			letterSpacing: 0.06,
		});
		this.hostnameText.position.set(0, 1.0, 0.01);
		this.hostnameText.visible = false;
		this.add(this.hostnameText);

		// Form fields — positions relative to panel center
		const fieldW = PANEL_W - 0.6; // 1.8

		// Username field
		this.userGroup = new THREE.Group();
		this.userGroup.position.set(-fieldW / 2, 0.55, 0.01);
		this.userGroup.visible = false;
		this.userInput = new TextInput3D({
			label: "Username",
			width: fieldW,
			interaction,
			onSubmit: () => this.focusPassword(),
			onTab: () => this.focusPassword(),
			onCapsLock: (on) => this.notifications.setCapsLock(on),
		});
		this.userGroup.add(this.userInput);
		this.add(this.userGroup);

		// Password field
		this.passGroup = new THREE.Group();
		this.passGroup.position.set(-fieldW / 2, 0.15, 0.01);
		this.passGroup.visible = false;
		this.passInput = new TextInput3D({
			label: "Password",
			width: fieldW,
			password: true,
			interaction,
			onSubmit: () => this.handleLogin(),
			onTab: () => this.userInput.focus(),
			onCapsLock: (on) => this.notifications.setCapsLock(on),
		});
		this.passGroup.add(this.passInput);
		this.add(this.passGroup);

		// Session picker
		this.sessionGroup = new THREE.Group();
		this.sessionGroup.position.set(-fieldW / 2, -0.25, 0.01);
		this.sessionGroup.visible = false;
		this.sessionPicker = new SessionPicker3D({
			width: fieldW,
			interaction,
		});
		this.sessionGroup.add(this.sessionPicker);
		this.add(this.sessionGroup);

		// Submit button
		this.submitGroup = new THREE.Group();
		this.submitGroup.position.set(0, -0.7, 0.01);
		this.submitGroup.visible = false;
		this.submitBtn = new Button3D({
			label: "ACCESS SYSTEM",
			width: fieldW,
			height: 0.2,
			fontSize: 0.055,
			onClick: () => this.handleLogin(),
			interaction,
		});
		this.submitGroup.add(this.submitBtn);
		this.add(this.submitGroup);

		// Notifications
		this.notifications = new Notifications3D();
		this.notifications.position.set(0, -1.05, 0.01);
		this.add(this.notifications);

		// Start invisible
		this.visible = false;
	}

	private addCornerBrackets(hw: number, hh: number) {
		const size = 0.12;
		const corners = [
			{ x: -hw, y: hh, dx: 1, dy: -1 },   // top-left
			{ x: hw, y: hh, dx: -1, dy: -1 },    // top-right
			{ x: -hw, y: -hh, dx: 1, dy: 1 },    // bottom-left
			{ x: hw, y: -hh, dx: -1, dy: 1 },    // bottom-right
		];

		const bracketMat = new THREE.LineBasicMaterial({
			color: 0x00ffff,
			transparent: true,
			opacity: 0.9,
			blending: THREE.AdditiveBlending,
		});

		for (const c of corners) {
			const pts = [
				c.x, c.y + c.dy * size, 0,
				c.x, c.y, 0,
				c.x + c.dx * size, c.y, 0,
			];
			const geo = new THREE.BufferGeometry();
			geo.setAttribute("position", new THREE.Float32BufferAttribute(pts, 3));
			const line = new THREE.Line(geo, bracketMat);
			line.position.z = 0.002;
			this.add(line);
		}
	}

	private focusPassword() {
		this.userInput.blur();
		this.passInput.focus();
	}

	private async handleLogin() {
		if (this.busy || !this.userInput.value || !this.sessions.length) return;
		this.notifications.clearError();
		this.busy = true;
		this.submitBtn.disabled = true;
		this.submitBtn.label = "AUTHENTICATING...";

		try {
			const session = this.sessions[this.sessionPicker.selected];
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
		this.submitBtn.label = "ACCESS SYSTEM";
		this.passInput.value = "";
		this.passInput.focus();

		this.notifications.showError(msg);
		this.callbacks.onError(msg);

		// Panel shake animation
		this.shakePanel();
		// Red flash on border
		this.flashBorder();
	}

	private async shakePanel() {
		const offsets = [0.04, -0.035, 0.025, -0.02, 0.01, 0];
		const baseX = this.position.x;
		const baseY = this.position.y;
		for (const off of offsets) {
			this.position.x = baseX + off;
			this.position.y = baseY + off * 0.3;
			await sleep(80);
		}
		this.position.x = baseX;
		this.position.y = baseY;
	}

	private async flashBorder() {
		const origColor = this.panelMat.borderColor.clone();
		this.panelMat.borderColor.set(0xff0044);
		this.panelMat.borderAlpha = 0.8;
		await sleep(300);
		this.panelMat.borderColor.copy(origColor);
		this.panelMat.borderAlpha = 0.3;
	}

	// Public API matching LoginOverlay's shape

	setSessions(sessions: Session[]) {
		this.sessions = sessions;
		this.sessionPicker.setSessions(sessions);
		this.sessionGroup.visible = sessions.length > 1;
	}

	setSelectedSession(index: number) {
		this.sessionPicker.setSelected(index);
	}

	setHostname(name: string) {
		if (name) {
			this.hostnameText.text = name;
			this.hostnameText.sync();
			this.hostnameText.visible = true;
		}
	}

	setUsername(name: string) {
		this.userInput.value = name;
	}

	async runBootSequence(): Promise<void> {
		this.visible = true;

		const lines = [
			"SYSTEM BOOT...",
			"ESTABLISHING CONNECTION...",
			"ACCESS TERMINAL READY",
		];

		for (const line of lines) {
			await this.typeBootText(line);
			await sleep(400);
		}

		this.callbacks.onBoot();

		this.bootText.text = "";
		this.bootText.sync();

		// Show hostname if set
		if (this.hostnameText.text) {
			this.hostnameText.visible = true;
		}

		// Staggered reveal of form elements
		const groups = [this.userGroup, this.passGroup];
		if (this.sessions.length > 1) groups.push(this.sessionGroup);

		for (const group of groups) {
			await sleep(100);
			group.visible = true;
			// Animate fade in + slide up
			const targetY = group.position.y;
			group.position.y = targetY - 0.05;
			tweenValue(group.position.y, targetY, 300, (v) => {
				group.position.y = v;
			});
		}

		await sleep(100);
		this.submitGroup.visible = true;
		// Focus username
		this.userInput.focus();
	}

	private async typeBootText(text: string): Promise<void> {
		this.bootText.text = "";
		for (let i = 0; i < text.length; i++) {
			this.bootText.text = text.slice(0, i + 1);
			this.bootText.sync();
			await sleep(30 + Math.random() * 20);
		}
	}
}
