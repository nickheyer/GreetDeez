import * as THREE from "three";

const EMBER_COUNT = 200;
const PORTAL_Z = -22;

export class HellPortal {
	readonly group: THREE.Group;
	private pentagram: THREE.Group;
	private embers: THREE.Points;
	private velocities: Float32Array;
	private pentagramMat: THREE.LineBasicMaterial;

	constructor() {
		this.group = new THREE.Group();

		// Floor pentagram — ring + star
		this.pentagram = new THREE.Group();
		this.pentagramMat = new THREE.LineBasicMaterial({
			color: 0xff2200,
			transparent: true,
			opacity: 0.15,
			blending: THREE.AdditiveBlending,
		});

		// Outer ring
		const ringGeo = new THREE.RingGeometry(1.8, 1.9, 32);
		const ringMat = new THREE.MeshBasicMaterial({
			color: 0xff2200,
			transparent: true,
			opacity: 0.1,
			blending: THREE.AdditiveBlending,
			side: THREE.DoubleSide,
			depthWrite: false,
		});
		const ring = new THREE.Mesh(ringGeo, ringMat);
		ring.rotation.x = -Math.PI / 2;
		this.pentagram.add(ring);

		// Star (pentagram lines)
		const starPts: number[] = [];
		for (let i = 0; i < 5; i++) {
			const a1 = (i * 2 * Math.PI) / 5 - Math.PI / 2;
			const a2 = (((i + 2) % 5) * 2 * Math.PI) / 5 - Math.PI / 2;
			starPts.push(Math.cos(a1) * 1.7, 0, Math.sin(a1) * 1.7);
			starPts.push(Math.cos(a2) * 1.7, 0, Math.sin(a2) * 1.7);
		}
		const starGeo = new THREE.BufferGeometry();
		starGeo.setAttribute("position", new THREE.Float32BufferAttribute(starPts, 3));
		const star = new THREE.LineSegments(starGeo, this.pentagramMat);
		this.pentagram.add(star);

		this.pentagram.position.set(0, 0.02, PORTAL_Z);
		this.group.add(this.pentagram);

		// Ember particles drifting upward (inverted rain)
		const positions = new Float32Array(EMBER_COUNT * 3);
		const colors = new Float32Array(EMBER_COUNT * 3);
		this.velocities = new Float32Array(EMBER_COUNT);

		const hot = new THREE.Color(0xff4400);
		const warm = new THREE.Color(0xffaa00);
		const red = new THREE.Color(0xff2200);
		const palette = [hot, hot, warm, red];

		for (let i = 0; i < EMBER_COUNT; i++) {
			const i3 = i * 3;
			positions[i3] = (Math.random() - 0.5) * 4;
			positions[i3 + 1] = Math.random() * 6;
			positions[i3 + 2] = PORTAL_Z + (Math.random() - 0.5) * 4;

			this.velocities[i] = 0.5 + Math.random() * 1.5;

			const c = palette[Math.floor(Math.random() * palette.length)];
			colors[i3] = c.r;
			colors[i3 + 1] = c.g;
			colors[i3 + 2] = c.b;
		}

		const emberGeo = new THREE.BufferGeometry();
		emberGeo.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
		emberGeo.setAttribute("color", new THREE.Float32BufferAttribute(colors, 3));

		const emberMat = new THREE.PointsMaterial({
			size: 0.04,
			vertexColors: true,
			transparent: true,
			opacity: 0.7,
			blending: THREE.AdditiveBlending,
			depthWrite: false,
		});

		this.embers = new THREE.Points(emberGeo, emberMat);
		this.group.add(this.embers);
	}

	update(time: number, dt: number) {
		// Pulse pentagram opacity
		this.pentagramMat.opacity = 0.1 + Math.sin(time * 1.5) * 0.05;

		// Drift embers upward
		const pos = this.embers.geometry.attributes.position as THREE.BufferAttribute;
		const arr = pos.array as Float32Array;

		for (let i = 0; i < EMBER_COUNT; i++) {
			const i3 = i * 3;
			arr[i3 + 1] += this.velocities[i] * dt;
			// Slight horizontal drift
			arr[i3] += Math.sin(time + i * 0.1) * 0.002;

			// Reset when too high
			if (arr[i3 + 1] > 6) {
				arr[i3 + 1] = 0;
				arr[i3] = (Math.random() - 0.5) * 4;
				arr[i3 + 2] = PORTAL_Z + (Math.random() - 0.5) * 4;
			}
		}

		pos.needsUpdate = true;
	}

	dispose() {
		this.group.traverse((obj) => {
			if (obj instanceof THREE.Mesh || obj instanceof THREE.Points) {
				obj.geometry.dispose();
				(obj.material as THREE.Material).dispose();
			}
		});
	}
}
