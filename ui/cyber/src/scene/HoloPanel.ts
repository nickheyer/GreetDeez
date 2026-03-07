import * as THREE from "three";

const COUNT = 4;

export class HoloPanel {
	readonly group: THREE.Group;
	private meshes: THREE.Mesh[] = [];

	constructor() {
		this.group = new THREE.Group();

		const mat = new THREE.MeshBasicMaterial({
			color: 0x00ffff,
			wireframe: true,
			transparent: true,
			opacity: 0.06,
		});

		for (let i = 0; i < COUNT; i++) {
			const radius = 0.5 + Math.random() * 0.8;
			const geo = new THREE.IcosahedronGeometry(radius, 1);
			const mesh = new THREE.Mesh(geo, mat.clone());

			const angle = (i / COUNT) * Math.PI * 2;
			const r = 2 + Math.random() * 1.5;
			mesh.position.set(
				Math.cos(angle) * r,
				1.5 + Math.random() * 2,
				Math.sin(angle) * r - 2,
			);

			mesh.userData.rotSpeed = {
				x: 0.1 + Math.random() * 0.2,
				y: 0.15 + Math.random() * 0.25,
			};

			this.group.add(mesh);
			this.meshes.push(mesh);
		}
	}

	update(time: number) {
		for (const mesh of this.meshes) {
			const s = mesh.userData.rotSpeed;
			mesh.rotation.x = time * s.x;
			mesh.rotation.y = time * s.y;
		}
	}

	dispose() {
		for (const mesh of this.meshes) {
			mesh.geometry.dispose();
			(mesh.material as THREE.Material).dispose();
		}
	}
}
