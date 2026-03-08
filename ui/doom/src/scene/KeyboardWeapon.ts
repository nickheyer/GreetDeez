import * as THREE from "three";

/** DOOM weapon-bob keyboard at screen bottom — attached to camera */
export class KeyboardWeapon {
	readonly group: THREE.Group;
	private walkBob = false;

	constructor() {
		this.group = new THREE.Group();

		const frameMat = new THREE.MeshBasicMaterial({ color: 0x2a2a2a });
		const keyMat = new THREE.MeshBasicMaterial({ color: 0x3a3a3a });

		const keyCapMat = new THREE.MeshBasicMaterial({
			color: 0xffaa00,
			transparent: true,
			opacity: 0.3,
		});

		// Keyboard frame
		const frame = new THREE.Mesh(
			new THREE.BoxGeometry(1.2, 0.06, 0.4),
			frameMat,
		);
		this.group.add(frame);

		// Key rows — merged for performance
		const keyGeos: THREE.BufferGeometry[] = [];
		const capGeos: THREE.BufferGeometry[] = [];

		const rows = [
			{ count: 14, z: -0.13, w: 0.065 },
			{ count: 13, z: -0.04, w: 0.065 },
			{ count: 12, z: 0.05, w: 0.065 },
			{ count: 10, z: 0.14, w: 0.065 },
		];

		for (const row of rows) {
			const startX = -(row.count * 0.075) / 2;
			for (let i = 0; i < row.count; i++) {
				const x = startX + i * 0.075 + 0.035;
				const key = new THREE.BoxGeometry(row.w, 0.03, 0.065);
				key.translate(x, 0.045, row.z);
				keyGeos.push(key);

				// Amber cap on some keys
				if (Math.random() < 0.3) {
					const cap = new THREE.PlaneGeometry(row.w * 0.6, 0.04);
					cap.translate(x, 0.062, row.z);
					capGeos.push(cap);
				}
			}
		}

		// Merge keys
		const mergedKeys = mergeGeos(keyGeos);
		if (mergedKeys) this.group.add(new THREE.Mesh(mergedKeys, keyMat));

		const mergedCaps = mergeGeos(capGeos);
		if (mergedCaps) this.group.add(new THREE.Mesh(mergedCaps, keyCapMat));

		// Position in screen space (bottom center, slightly tilted)
		this.group.position.set(0, -0.55, -0.8);
		this.group.rotation.x = -0.3;
	}

	setWalkBob(enabled: boolean) {
		this.walkBob = enabled;
	}

	update(time: number) {
		if (this.walkBob) {
			// Walk cycle: faster, more pronounced
			this.group.position.y = -0.55 + Math.sin(time * 8.0) * 0.03;
			this.group.position.x = Math.cos(time * 4.0) * 0.015;
		} else {
			// Idle: subtle breathing bob
			this.group.position.y = -0.55 + Math.sin(time * 1.5) * 0.005;
			this.group.position.x = Math.sin(time * 0.7) * 0.003;
		}
	}

	dispose() {
		this.group.traverse((obj) => {
			if (obj instanceof THREE.Mesh) {
				obj.geometry.dispose();
				(obj.material as THREE.Material).dispose();
			}
		});
	}
}

function mergeGeos(geos: THREE.BufferGeometry[]): THREE.BufferGeometry | null {
	if (geos.length === 0) return null;

	let totalVerts = 0;
	let totalIdx = 0;

	for (const g of geos) {
		const pos = g.getAttribute("position");
		totalVerts += pos.count;
		totalIdx += g.index ? g.index.count : pos.count;
	}

	const positions = new Float32Array(totalVerts * 3);
	const indices = new Uint32Array(totalIdx);
	let vertOffset = 0;
	let idxOffset = 0;

	for (const g of geos) {
		const pos = g.getAttribute("position") as THREE.BufferAttribute;
		positions.set(pos.array as Float32Array, vertOffset * 3);

		if (g.index) {
			const idx = g.index.array;
			for (let i = 0; i < idx.length; i++) {
				indices[idxOffset + i] = idx[i] + vertOffset;
			}
			idxOffset += idx.length;
		} else {
			for (let i = 0; i < pos.count; i++) {
				indices[idxOffset + i] = vertOffset + i;
			}
			idxOffset += pos.count;
		}

		vertOffset += pos.count;
		g.dispose();
	}

	const merged = new THREE.BufferGeometry();
	merged.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
	merged.setIndex(new THREE.BufferAttribute(indices, 1));
	return merged;
}
