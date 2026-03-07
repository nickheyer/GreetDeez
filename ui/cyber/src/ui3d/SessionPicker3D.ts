import * as THREE from "three";
import { createText } from "./text-helper.js";
import type { SceneInteraction } from "./SceneInteraction.js";

interface Session {
	name: string;
	cmd: string[];
	type: number;
	desktop: string;
}

export class SessionPicker3D extends THREE.Group {
	private triggerGroup: THREE.Group;
	private triggerText: ReturnType<typeof createText>;
	private underline: THREE.Line;
	private underlineMat: THREE.LineBasicMaterial;
	private menuGroup: THREE.Group | null = null;
	private interaction: SceneInteraction;
	private _width: number;

	private sessions: Session[] = [];
	private selectedIndex = 0;
	private onSelect?: (index: number) => void;
	private hitPlane: THREE.Mesh;

	constructor(opts: {
		width: number;
		interaction: SceneInteraction;
		onSelect?: (index: number) => void;
	}) {
		super();
		this._width = opts.width;
		this.interaction = opts.interaction;
		this.onSelect = opts.onSelect;

		// Label
		const label = createText({
			text: "SESSION",
			fontSize: 0.055,
			color: 0x8888aa,
			letterSpacing: 0.04,
		});
		label.position.set(0, 0.1, 0);
		this.add(label);

		// Trigger group
		this.triggerGroup = new THREE.Group();
		this.add(this.triggerGroup);

		this.triggerText = createText({
			text: "---",
			fontSize: 0.07,
			color: 0xe0e0ff,
		});
		this.triggerGroup.add(this.triggerText);

		// Underline
		const lineGeo = new THREE.BufferGeometry();
		lineGeo.setAttribute("position", new THREE.Float32BufferAttribute([
			0, -0.05, 0, opts.width, -0.05, 0,
		], 3));
		this.underlineMat = new THREE.LineBasicMaterial({
			color: 0x00ffff,
			transparent: true,
			opacity: 0.3,
		});
		this.underline = new THREE.Line(lineGeo, this.underlineMat);
		this.triggerGroup.add(this.underline);

		// Hit plane
		const hitGeo = new THREE.PlaneGeometry(opts.width, 0.2);
		const hitMat = new THREE.MeshBasicMaterial({ visible: false });
		this.hitPlane = new THREE.Mesh(hitGeo, hitMat);
		this.hitPlane.position.set(opts.width / 2, 0, 0);
		this.triggerGroup.add(this.hitPlane);

		// Interaction
		opts.interaction.register(this.triggerGroup, {
			onClick: () => this.toggle(),
			onHover: (h) => {
				this.underlineMat.opacity = h ? 1.0 : 0.3;
			},
		});
	}

	setSessions(sessions: Session[]) {
		this.sessions = sessions;
		this.visible = sessions.length > 1;
		if (sessions.length > 0) {
			this.triggerText.text = sessions[this.selectedIndex]?.name ?? "---";
			this.triggerText.sync();
		}
	}

	setSelected(index: number) {
		this.selectedIndex = index;
		if (this.sessions.length > 0) {
			this.triggerText.text = this.sessions[index]?.name ?? "---";
			this.triggerText.sync();
		}
	}

	get selected(): number {
		return this.selectedIndex;
	}

	private toggle() {
		if (this.menuGroup) {
			this.closeMenu();
		} else {
			this.openMenu();
		}
	}

	private openMenu() {
		this.menuGroup = new THREE.Group();
		this.menuGroup.position.set(0, 0.15, 0.01);

		// Background
		const menuH = this.sessions.length * 0.12 + 0.04;
		const bgGeo = new THREE.PlaneGeometry(this._width, menuH);
		const bgMat = new THREE.MeshBasicMaterial({
			color: 0x020208,
			transparent: true,
			opacity: 0.95,
			depthWrite: false,
		});
		const bg = new THREE.Mesh(bgGeo, bgMat);
		bg.position.set(this._width / 2, menuH / 2, 0);
		this.menuGroup.add(bg);

		// Border
		const hw = this._width / 2;
		const pts = [
			0, 0, 0, this._width, 0, 0,
			this._width, 0, 0, this._width, menuH, 0,
			this._width, menuH, 0, 0, menuH, 0,
			0, menuH, 0, 0, 0, 0,
		];
		const borderGeo = new THREE.BufferGeometry();
		borderGeo.setAttribute("position", new THREE.Float32BufferAttribute(pts, 3));
		const borderMat = new THREE.LineBasicMaterial({
			color: 0x00ffff,
			transparent: true,
			opacity: 0.3,
		});
		this.menuGroup.add(new THREE.LineSegments(borderGeo, borderMat));

		// Items
		this.sessions.forEach((s, i) => {
			const itemText = createText({
				text: s.name,
				fontSize: 0.06,
				color: i === this.selectedIndex ? 0x00ffff : 0xe0e0ff,
			});
			const y = 0.06 + i * 0.12;
			itemText.position.set(0.06, y, 0.001);

			// Hit plane per item
			const itemHitGeo = new THREE.PlaneGeometry(this._width, 0.12);
			const itemHitMat = new THREE.MeshBasicMaterial({ visible: false });
			const itemHit = new THREE.Mesh(itemHitGeo, itemHitMat);
			itemHit.position.set(this._width / 2, y, 0);

			const itemGroup = new THREE.Group();
			itemGroup.add(itemText);
			itemGroup.add(itemHit);
			this.menuGroup!.add(itemGroup);

			this.interaction.register(itemGroup, {
				onClick: () => {
					this.selectedIndex = i;
					this.triggerText.text = s.name;
					this.triggerText.sync();
					this.onSelect?.(i);
					this.closeMenu();
				},
				onHover: (h) => {
					itemText.color = h ? 0x00ffff : (i === this.selectedIndex ? 0x00ffff : 0xe0e0ff);
				},
			});
		});

		this.add(this.menuGroup);
	}

	private closeMenu() {
		if (!this.menuGroup) return;
		// Unregister all item groups
		this.menuGroup.children.forEach((child) => {
			this.interaction.unregister(child);
		});
		this.remove(this.menuGroup);
		this.menuGroup = null;
	}
}
