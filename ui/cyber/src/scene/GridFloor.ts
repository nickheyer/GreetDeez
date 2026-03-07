import * as THREE from "three";

export class GridFloor {
	readonly mesh: THREE.LineSegments;
	private scrollOffset = 0;

	constructor() {
		const size = 200;
		const divisions = 80;
		const half = size / 2;
		const step = size / divisions;

		const vertices: number[] = [];

		// Lines along X axis
		for (let i = 0; i <= divisions; i++) {
			const z = -half + i * step;
			vertices.push(-half, 0, z, half, 0, z);
		}

		// Lines along Z axis
		for (let i = 0; i <= divisions; i++) {
			const x = -half + i * step;
			vertices.push(x, 0, -half, x, 0, half);
		}

		const geo = new THREE.BufferGeometry();
		geo.setAttribute("position", new THREE.Float32BufferAttribute(vertices, 3));

		const mat = new THREE.LineBasicMaterial({
			color: 0x00ffff,
			transparent: true,
			opacity: 0.15,
		});

		this.mesh = new THREE.LineSegments(geo, mat);
	}

	update(time: number) {
		// Scroll toward camera
		this.scrollOffset = (time * 0.8) % 2.5;
		this.mesh.position.z = this.scrollOffset;

		// Brightness pulse
		const mat = this.mesh.material as THREE.LineBasicMaterial;
		mat.opacity = 0.12 + 0.06 * Math.sin(time * 0.5);
	}

	dispose() {
		this.mesh.geometry.dispose();
		(this.mesh.material as THREE.Material).dispose();
	}
}
