import * as THREE from "three";
import { createText } from "./text-helper.js";
import type { SceneInteraction } from "./SceneInteraction.js";

export interface Button3DOptions {
	label: string;
	width: number;
	height: number;
	fontSize?: number;
	onClick: () => void;
	interaction: SceneInteraction;
}

export class Button3D extends THREE.Group {
	private bg: THREE.Mesh;
	private border: THREE.LineSegments;
	private borderMat: THREE.LineBasicMaterial;
	private labelText: ReturnType<typeof createText>;
	private _disabled = false;

	constructor(opts: Button3DOptions) {
		super();

		const w = opts.width;
		const h = opts.height;

		// Background plane
		const bgGeo = new THREE.PlaneGeometry(w, h);
		const bgMat = new THREE.MeshBasicMaterial({
			color: 0x0a0a05,
			transparent: true,
			opacity: 0.6,
			depthWrite: false,
		});
		this.bg = new THREE.Mesh(bgGeo, bgMat);
		this.add(this.bg);

		// Border — amber
		const hw = w / 2;
		const hh = h / 2;
		const pts = [
			-hw, -hh, 0, hw, -hh, 0,
			hw, -hh, 0, hw, hh, 0,
			hw, hh, 0, -hw, hh, 0,
			-hw, hh, 0, -hw, -hh, 0,
		];
		const borderGeo = new THREE.BufferGeometry();
		borderGeo.setAttribute("position", new THREE.Float32BufferAttribute(pts, 3));
		this.borderMat = new THREE.LineBasicMaterial({
			color: 0xffaa00,
			transparent: true,
			opacity: 0.6,
		});
		this.border = new THREE.LineSegments(borderGeo, this.borderMat);
		this.border.position.z = 0.001;
		this.add(this.border);

		// Label — amber
		this.labelText = createText({
			text: opts.label,
			fontSize: opts.fontSize ?? 0.06,
			color: 0xffaa00,
			anchorX: "center",
			anchorY: "middle",
			letterSpacing: 0.06,
		});
		this.labelText.position.z = 0.002;
		this.add(this.labelText);

		// Interaction
		opts.interaction.register(this, {
			onClick: () => {
				if (this._disabled) return;
				this.borderMat.opacity = 1.0;
				(this.bg.material as THREE.MeshBasicMaterial).opacity = 0.3;
				setTimeout(() => {
					this.borderMat.opacity = 0.6;
					(this.bg.material as THREE.MeshBasicMaterial).opacity = 0.6;
				}, 120);
				opts.onClick();
			},
			onHover: (hovered) => {
				if (this._disabled) return;
				this.borderMat.opacity = hovered ? 1.0 : 0.6;
				(this.bg.material as THREE.MeshBasicMaterial).opacity = hovered ? 0.15 : 0.6;
			},
		});
	}

	set label(text: string) {
		this.labelText.text = text;
		this.labelText.sync();
	}

	get disabled(): boolean {
		return this._disabled;
	}

	set disabled(v: boolean) {
		this._disabled = v;
		const a = v ? 0.3 : 1.0;
		this.labelText.material.opacity = a;
		this.borderMat.opacity = v ? 0.2 : 0.6;
		(this.bg.material as THREE.MeshBasicMaterial).opacity = v ? 0.15 : 0.6;
	}
}
