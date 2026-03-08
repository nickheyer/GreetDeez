import * as THREE from "three";

const CORRIDOR_LENGTH = 30;
const CORRIDOR_WIDTH = 4;
const CORRIDOR_HEIGHT = 4;
const GRATE_GAP = 0.15;

/** Procedural stone/metal corridor — static geometry, no update needed */
export class DoomCorridor {
	readonly group: THREE.Group;

	constructor() {
		this.group = new THREE.Group();

		const wallGeos: THREE.BufferGeometry[] = [];
		const panelGeos: THREE.BufferGeometry[] = [];
		const grateGeos: THREE.BufferGeometry[] = [];

		const wallMat = new THREE.MeshStandardMaterial({
			color: 0x3a3a3a,
			roughness: 0.85,
			metalness: 0.1,
			emissive: 0x111111,
			emissiveIntensity: 0.3,
			side: THREE.DoubleSide,
		});
		const panelMat = new THREE.MeshStandardMaterial({
			color: 0x4a2010,
			roughness: 0.7,
			metalness: 0.2,
			emissive: 0x1a0800,
			emissiveIntensity: 0.4,
			side: THREE.DoubleSide,
		});
		const grateMat = new THREE.MeshStandardMaterial({
			color: 0x2a2a2a,
			roughness: 0.6,
			metalness: 0.4,
			emissive: 0x0a0a0a,
			emissiveIntensity: 0.3,
			side: THREE.DoubleSide,
		});

		const hw = CORRIDOR_WIDTH / 2;

		// Ceiling
		const ceiling = new THREE.BoxGeometry(CORRIDOR_WIDTH, 0.3, CORRIDOR_LENGTH);
		ceiling.translate(0, CORRIDOR_HEIGHT + 0.15, -CORRIDOR_LENGTH / 2 + 1);
		wallGeos.push(ceiling);

		// Left wall
		const leftWall = new THREE.BoxGeometry(0.3, CORRIDOR_HEIGHT, CORRIDOR_LENGTH);
		leftWall.translate(-hw - 0.15, CORRIDOR_HEIGHT / 2, -CORRIDOR_LENGTH / 2 + 1);
		wallGeos.push(leftWall);

		// Right wall
		const rightWall = new THREE.BoxGeometry(0.3, CORRIDOR_HEIGHT, CORRIDOR_LENGTH);
		rightWall.translate(hw + 0.15, CORRIDOR_HEIGHT / 2, -CORRIDOR_LENGTH / 2 + 1);
		wallGeos.push(rightWall);

		// Back wall (far end)
		const backWall = new THREE.BoxGeometry(CORRIDOR_WIDTH + 0.6, CORRIDOR_HEIGHT + 0.3, 0.3);
		backWall.translate(0, CORRIDOR_HEIGHT / 2, -CORRIDOR_LENGTH + 1 - 0.15);
		wallGeos.push(backWall);

		// Floor grate segments — alternating solid/gap for lava visibility
		const segLen = 2.0;
		for (let z = 1; z > -CORRIDOR_LENGTH + 1; z -= segLen) {
			// Solid grate strip
			const grate = new THREE.BoxGeometry(CORRIDOR_WIDTH - GRATE_GAP * 2, 0.08, segLen * 0.7);
			grate.translate(0, 0, z - segLen * 0.35);
			grateGeos.push(grate);

			// Side rails
			const leftRail = new THREE.BoxGeometry(GRATE_GAP, 0.12, segLen);
			leftRail.translate(-hw + GRATE_GAP / 2, 0, z - segLen / 2);
			grateGeos.push(leftRail);

			const rightRail = new THREE.BoxGeometry(GRATE_GAP, 0.12, segLen);
			rightRail.translate(hw - GRATE_GAP / 2, 0, z - segLen / 2);
			grateGeos.push(rightRail);
		}

		// Inset wall panels for tech detail (both sides)
		for (let z = 0; z > -CORRIDOR_LENGTH + 2; z -= 3) {
			// Left panel
			const lp = new THREE.BoxGeometry(0.05, 1.2, 1.5);
			lp.translate(-hw + 0.025, CORRIDOR_HEIGHT / 2, z - 0.75);
			panelGeos.push(lp);

			// Right panel
			const rp = new THREE.BoxGeometry(0.05, 1.2, 1.5);
			rp.translate(hw - 0.025, CORRIDOR_HEIGHT / 2, z - 0.75);
			panelGeos.push(rp);
		}

		// Merge geometries
		this.addMerged(wallGeos, wallMat);
		this.addMerged(panelGeos, panelMat);
		this.addMerged(grateGeos, grateMat);
	}

	private addMerged(geos: THREE.BufferGeometry[], mat: THREE.Material) {
		if (geos.length === 0) return;
		const merged = mergeGeometries(geos);
		if (merged) {
			this.group.add(new THREE.Mesh(merged, mat));
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
	const normals = new Float32Array(totalVerts * 3);
	const indices = new Uint32Array(totalIdx);
	let vertOffset = 0;
	let idxOffset = 0;

	for (const g of geos) {
		const pos = g.getAttribute("position") as THREE.BufferAttribute;
		const norm = g.getAttribute("normal") as THREE.BufferAttribute | null;
		positions.set(pos.array as Float32Array, vertOffset * 3);
		if (norm) {
			normals.set(norm.array as Float32Array, vertOffset * 3);
		}

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
	merged.setAttribute("normal", new THREE.Float32BufferAttribute(normals, 3));
	merged.setIndex(new THREE.BufferAttribute(indices, 1));
	return merged;
}
