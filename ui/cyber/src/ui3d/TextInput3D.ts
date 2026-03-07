import * as THREE from "three";
import { createText } from "./text-helper.js";
import type { SceneInteraction } from "./SceneInteraction.js";

export interface TextInput3DOptions {
	label: string;
	width: number;
	password?: boolean;
	interaction: SceneInteraction;
	onSubmit?: () => void;
	onTab?: () => void;
	onCapsLock?: (on: boolean) => void;
}

export class TextInput3D extends THREE.Group {
	private static activeInput: TextInput3D | null = null;

	private buffer = "";
	private isPassword: boolean;
	private labelText: ReturnType<typeof createText>;
	private valueText: ReturnType<typeof createText>;
	private underline: THREE.Line;
	private underlineMat: THREE.LineBasicMaterial;
	private cursor: THREE.Mesh;
	private selectionHighlight: THREE.Mesh;
	private selectionMat: THREE.MeshBasicMaterial;
	private cursorBlinkId = 0;
	private _focused = false;
	private _selected = false;
	private _width: number;
	private onSubmit?: () => void;
	private onTab?: () => void;
	private onCapsLock?: (on: boolean) => void;
	private keyHandler: ((e: KeyboardEvent) => void) | null = null;
	private hitPlane: THREE.Mesh;

	constructor(opts: TextInput3DOptions) {
		super();

		this._width = opts.width;
		this.isPassword = opts.password ?? false;
		this.onSubmit = opts.onSubmit;
		this.onTab = opts.onTab;
		this.onCapsLock = opts.onCapsLock;

		// Label
		this.labelText = createText({
			text: opts.label.toUpperCase(),
			fontSize: 0.055,
			color: 0x8888aa,
			letterSpacing: 0.04,
		});
		this.labelText.position.set(0, 0.1, 0);
		this.add(this.labelText);

		// Selection highlight (behind text)
		this.selectionMat = new THREE.MeshBasicMaterial({
			color: 0x00ffff,
			transparent: true,
			opacity: 0,
			depthWrite: false,
		});
		this.selectionHighlight = new THREE.Mesh(
			new THREE.PlaneGeometry(1, 0.09),
			this.selectionMat,
		);
		this.selectionHighlight.position.set(0, 0, -0.001);
		this.add(this.selectionHighlight);

		// Value text
		this.valueText = createText({
			text: "",
			fontSize: 0.07,
			color: 0xe0e0ff,
		});
		this.valueText.position.set(0, 0, 0);
		this.add(this.valueText);

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
		this.add(this.underline);

		// Cursor
		const cursorGeo = new THREE.PlaneGeometry(0.005, 0.08);
		const cursorMat = new THREE.MeshBasicMaterial({
			color: 0x00ffff,
			transparent: true,
			opacity: 0,
			depthWrite: false,
		});
		this.cursor = new THREE.Mesh(cursorGeo, cursorMat);
		this.cursor.position.set(0, 0, 0.001);
		this.add(this.cursor);

		// Invisible hit plane for raycasting
		const hitGeo = new THREE.PlaneGeometry(opts.width, 0.2);
		const hitMat = new THREE.MeshBasicMaterial({
			visible: false,
		});
		this.hitPlane = new THREE.Mesh(hitGeo, hitMat);
		this.hitPlane.position.set(opts.width / 2, 0, 0);
		this.add(this.hitPlane);

		// Interaction
		opts.interaction.register(this, {
			onClick: () => this.focus(),
		});
	}

	get value(): string {
		return this.buffer;
	}

	set value(v: string) {
		this.buffer = v;
		this.clearSelection();
		this.updateDisplay();
	}

	get focused(): boolean {
		return this._focused;
	}

	focus() {
		if (this._focused) return;
		// Blur whichever input was previously focused
		if (TextInput3D.activeInput && TextInput3D.activeInput !== this) {
			TextInput3D.activeInput.blur();
		}
		TextInput3D.activeInput = this;
		this._focused = true;
		this.underlineMat.opacity = 1.0;
		this.startCursorBlink();
		this.attachKeyboard();
	}

	blur() {
		if (!this._focused) return;
		if (TextInput3D.activeInput === this) TextInput3D.activeInput = null;
		this._focused = false;
		this.underlineMat.opacity = 0.3;
		this.clearSelection();
		this.stopCursorBlink();
		this.detachKeyboard();
	}

	private selectAll() {
		if (this.buffer.length === 0) return;
		this._selected = true;
		this.stopCursorBlink();
		this.updateSelectionHighlight();
	}

	private clearSelection() {
		if (!this._selected) return;
		this._selected = false;
		this.selectionMat.opacity = 0;
		if (this._focused) this.startCursorBlink();
	}

	private deleteSelection() {
		this.buffer = "";
		this._selected = false;
		this.selectionMat.opacity = 0;
		this.updateDisplay();
		this.startCursorBlink();
	}

	private updateSelectionHighlight() {
		const w = Math.min(this.getTextWidth(), this._width);
		this.selectionHighlight.geometry.dispose();
		this.selectionHighlight.geometry = new THREE.PlaneGeometry(w, 0.09);
		this.selectionHighlight.position.x = w / 2;
		this.selectionMat.opacity = 0.15;
	}

	private getTextWidth(): number {
		const geo = this.valueText.geometry;
		geo.computeBoundingBox();
		return geo.boundingBox ? geo.boundingBox.max.x : 0;
	}

	private updateDisplay() {
		const display = this.isPassword ? "●".repeat(this.buffer.length) : this.buffer;
		this.valueText.text = display;
		this.valueText.sync(() => {
			const textEnd = this.getTextWidth();

			// If text overflows the field, trim last char and re-render
			if (textEnd > this._width && this.buffer.length > 0) {
				this.buffer = this.buffer.slice(0, -1);
				this.updateDisplay();
				return;
			}

			this.cursor.position.x = textEnd;
		});
	}

	private startCursorBlink() {
		this.stopCursorBlink();
		const mat = this.cursor.material as THREE.MeshBasicMaterial;
		mat.opacity = 1.0;
		this.cursorBlinkId = window.setInterval(() => {
			mat.opacity = mat.opacity > 0.5 ? 0 : 1.0;
		}, 600);
	}

	private stopCursorBlink() {
		clearInterval(this.cursorBlinkId);
		(this.cursor.material as THREE.MeshBasicMaterial).opacity = 0;
	}

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
		if (!this._focused) return;

		this.onCapsLock?.(e.getModifierState("CapsLock"));

		// Ctrl/Cmd+A → select all
		if ((e.ctrlKey || e.metaKey) && e.key === "a") {
			e.preventDefault();
			this.selectAll();
			return;
		}

		if (e.key === "Enter") {
			e.preventDefault();
			this.onSubmit?.();
			return;
		}

		if (e.key === "Tab") {
			e.preventDefault();
			this.onTab?.();
			return;
		}

		if (e.key === "Backspace") {
			e.preventDefault();
			if (this._selected) {
				this.deleteSelection();
			} else if (this.buffer.length > 0) {
				this.buffer = this.buffer.slice(0, -1);
				this.updateDisplay();
			}
			return;
		}

		// Ignore other control/alt/meta combos
		if (e.ctrlKey || e.altKey || e.metaKey) return;
		if (e.key.length !== 1) return;

		// If text is selected, replace it
		if (this._selected) {
			this.buffer = "";
			this._selected = false;
			this.selectionMat.opacity = 0;
			this.startCursorBlink();
		}

		this.buffer += e.key;
		this.updateDisplay();
	}

	dispose() {
		this.detachKeyboard();
		this.stopCursorBlink();
	}
}
