import * as THREE from "three";

const PARTICLE_COUNT = 2500;
const SPREAD_X = 30;
const SPREAD_Z = 30;
const HEIGHT = 25;

export class ParticleRain {
	readonly points: THREE.Points;
	private velocities: Float32Array;

	constructor() {
		const positions = new Float32Array(PARTICLE_COUNT * 3);
		const colors = new Float32Array(PARTICLE_COUNT * 3);
		this.velocities = new Float32Array(PARTICLE_COUNT);

		const cyan = new THREE.Color(0x00ffff);
		const magenta = new THREE.Color(0xff00ff);
		const white = new THREE.Color(0xe0e0ff);
		const palette = [cyan, cyan, cyan, magenta, white];

		for (let i = 0; i < PARTICLE_COUNT; i++) {
			const i3 = i * 3;
			positions[i3] = (Math.random() - 0.5) * SPREAD_X;
			positions[i3 + 1] = Math.random() * HEIGHT;
			positions[i3 + 2] = (Math.random() - 0.5) * SPREAD_Z;

			this.velocities[i] = 1.5 + Math.random() * 2.5;

			const c = palette[Math.floor(Math.random() * palette.length)];
			colors[i3] = c.r;
			colors[i3 + 1] = c.g;
			colors[i3 + 2] = c.b;
		}

		const geo = new THREE.BufferGeometry();
		geo.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
		geo.setAttribute("color", new THREE.Float32BufferAttribute(colors, 3));

		const mat = new THREE.PointsMaterial({
			size: 0.06,
			vertexColors: true,
			transparent: true,
			opacity: 0.6,
			blending: THREE.AdditiveBlending,
			depthWrite: false,
		});

		this.points = new THREE.Points(geo, mat);
	}

	update(dt: number) {
		const pos = this.points.geometry.attributes.position as THREE.BufferAttribute;
		const arr = pos.array as Float32Array;

		for (let i = 0; i < PARTICLE_COUNT; i++) {
			const i3 = i * 3;
			arr[i3 + 1] -= this.velocities[i] * dt;

			if (arr[i3 + 1] < -1) {
				arr[i3 + 1] = HEIGHT;
				arr[i3] = (Math.random() - 0.5) * SPREAD_X;
				arr[i3 + 2] = (Math.random() - 0.5) * SPREAD_Z;
			}
		}

		pos.needsUpdate = true;
	}

	dispose() {
		this.points.geometry.dispose();
		(this.points.material as THREE.Material).dispose();
	}
}
