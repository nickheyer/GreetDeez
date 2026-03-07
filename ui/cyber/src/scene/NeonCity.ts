import * as THREE from "three";

export class NeonCity {
	readonly group: THREE.Group;

	constructor() {
		this.group = new THREE.Group();

		const buildingGeo: THREE.BufferGeometry[] = [];
		const windowGeo: THREE.BufferGeometry[] = [];

		const buildingMat = new THREE.MeshBasicMaterial({
			color: 0x040412,
			transparent: true,
			opacity: 0.9,
		});

		const windowMat = new THREE.MeshBasicMaterial({
			color: 0x0066ff,
			transparent: true,
			opacity: 0.4,
		});

		for (let i = 0; i < 50; i++) {
			const w = 1 + Math.random() * 3;
			const h = 4 + Math.random() * 16;
			const d = 1 + Math.random() * 3;
			const x = (Math.random() - 0.5) * 80;
			const z = -30 - Math.random() * 30;

			const box = new THREE.BoxGeometry(w, h, d);
			box.translate(x, h / 2, z);
			buildingGeo.push(box);

			// Emissive window planes on front face
			const winCount = Math.floor(h / 2);
			for (let row = 0; row < winCount; row++) {
				if (Math.random() < 0.4) continue; // some windows off
				const winW = w * 0.6;
				const winH = 0.4;
				const wy = row * 2 + 1.5;
				const plane = new THREE.PlaneGeometry(winW, winH);
				plane.translate(x, wy, z + d / 2 + 0.01);
				windowGeo.push(plane);
			}
		}

		// Merge into single draw calls
		if (buildingGeo.length > 0) {
			const merged = mergeGeometries(buildingGeo);
			if (merged) {
				this.group.add(new THREE.Mesh(merged, buildingMat));
			}
		}

		if (windowGeo.length > 0) {
			const merged = mergeGeometries(windowGeo);
			if (merged) {
				this.group.add(new THREE.Mesh(merged, windowMat));
			}
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

/** Simple geometry merge (no deps on BufferGeometryUtils) */
function mergeGeometries(geos: THREE.BufferGeometry[]): THREE.BufferGeometry | null {
	if (geos.length === 0) return null;

	let totalVerts = 0;
	let totalIdx = 0;

	for (const g of geos) {
		const pos = g.getAttribute("position");
		totalVerts += pos.count;
		if (g.index) {
			totalIdx += g.index.count;
		} else {
			totalIdx += pos.count;
		}
	}

	const positions = new Float32Array(totalVerts * 3);
	const indices = new Uint32Array(totalIdx);
	let vertOffset = 0;
	let idxOffset = 0;

	for (const g of geos) {
		const pos = g.getAttribute("position") as THREE.BufferAttribute;
		const arr = pos.array as Float32Array;

		positions.set(arr, vertOffset * 3);

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
