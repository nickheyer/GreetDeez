import * as THREE from "three";

interface InteractableEntry {
	onClick?: () => void;
	onHover?: (hovered: boolean) => void;
}

export class SceneInteraction {
	private raycaster = new THREE.Raycaster();
	private pointer = new THREE.Vector2();
	private registry = new Map<THREE.Object3D, InteractableEntry>();
	private canvas: HTMLCanvasElement;
	private camera: THREE.Camera;
	private hoveredObj: THREE.Object3D | null = null;

	constructor(canvas: HTMLCanvasElement, camera: THREE.Camera) {
		this.canvas = canvas;
		this.camera = camera;

		canvas.addEventListener("pointerdown", this.onPointerDown);
		canvas.addEventListener("pointermove", this.onPointerMove);
	}

	register(obj: THREE.Object3D, handlers: InteractableEntry) {
		this.registry.set(obj, handlers);
	}

	unregister(obj: THREE.Object3D) {
		this.registry.delete(obj);
	}

	private updatePointer(e: PointerEvent) {
		const rect = this.canvas.getBoundingClientRect();
		this.pointer.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
		this.pointer.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;
	}

	private findHandler(intersects: THREE.Intersection[]): { obj: THREE.Object3D; entry: InteractableEntry } | null {
		for (const hit of intersects) {
			let current: THREE.Object3D | null = hit.object;
			while (current) {
				const entry = this.registry.get(current);
				if (entry) return { obj: current, entry };
				current = current.parent;
			}
		}
		return null;
	}

	private onPointerDown = (e: PointerEvent) => {
		this.updatePointer(e);
		this.raycaster.setFromCamera(this.pointer, this.camera);

		const targets = this.getTargetMeshes();
		const intersects = this.raycaster.intersectObjects(targets, false);
		const found = this.findHandler(intersects);

		if (found?.entry.onClick) {
			found.entry.onClick();
		}
	};

	private onPointerMove = (e: PointerEvent) => {
		this.updatePointer(e);
		this.raycaster.setFromCamera(this.pointer, this.camera);

		const targets = this.getTargetMeshes();
		const intersects = this.raycaster.intersectObjects(targets, false);
		const found = this.findHandler(intersects);

		const newHovered = found?.obj ?? null;
		if (newHovered !== this.hoveredObj) {
			if (this.hoveredObj) {
				this.registry.get(this.hoveredObj)?.onHover?.(false);
			}
			if (newHovered && found) {
				found.entry.onHover?.(true);
			}
			this.hoveredObj = newHovered;
			this.canvas.style.cursor = newHovered ? "pointer" : "";
		}
	};

	private getTargetMeshes(): THREE.Object3D[] {
		const meshes: THREE.Object3D[] = [];
		for (const obj of this.registry.keys()) {
			obj.traverse((child) => {
				if ((child as THREE.Mesh).isMesh) {
					meshes.push(child);
				}
			});
		}
		return meshes;
	}

	dispose() {
		this.canvas.removeEventListener("pointerdown", this.onPointerDown);
		this.canvas.removeEventListener("pointermove", this.onPointerMove);
		this.registry.clear();
	}
}
