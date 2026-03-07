import * as THREE from "three";

const ARC_COUNT = 7;
const POINTS_PER_ARC = 60;

export class DataStream {
	readonly group: THREE.Group;
	private arcs: THREE.Line[] = [];
	private arcData: { radius: number; height: number; speed: number; phase: number }[] = [];

	constructor() {
		this.group = new THREE.Group();

		for (let i = 0; i < ARC_COUNT; i++) {
			const radius = 3 + Math.random() * 3;
			const height = 1 + Math.random() * 3;
			const speed = 0.2 + Math.random() * 0.3;
			const phase = Math.random() * Math.PI * 2;

			this.arcData.push({ radius, height, speed, phase });

			const positions = new Float32Array(POINTS_PER_ARC * 3);
			const colors = new Float32Array(POINTS_PER_ARC * 3);

			const geo = new THREE.BufferGeometry();
			geo.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
			geo.setAttribute("color", new THREE.Float32BufferAttribute(colors, 3));

			const mat = new THREE.LineBasicMaterial({
				vertexColors: true,
				transparent: true,
				opacity: 0.4,
				blending: THREE.AdditiveBlending,
			});

			const line = new THREE.Line(geo, mat);
			this.group.add(line);
			this.arcs.push(line);
		}
	}

	update(time: number) {
		const cyan = new THREE.Color(0x00ffff);
		const magenta = new THREE.Color(0xff00ff);

		for (let a = 0; a < ARC_COUNT; a++) {
			const d = this.arcData[a];
			const arc = this.arcs[a];
			const pos = arc.geometry.attributes.position as THREE.BufferAttribute;
			const col = arc.geometry.attributes.color as THREE.BufferAttribute;
			const pArr = pos.array as Float32Array;
			const cArr = col.array as Float32Array;

			const angleOffset = time * d.speed + d.phase;

			for (let i = 0; i < POINTS_PER_ARC; i++) {
				const t = i / (POINTS_PER_ARC - 1);
				const angle = angleOffset + t * Math.PI * 0.8;
				const i3 = i * 3;

				pArr[i3] = Math.cos(angle) * d.radius;
				pArr[i3 + 1] = d.height + Math.sin(t * Math.PI) * 1.5;
				pArr[i3 + 2] = Math.sin(angle) * d.radius;

				// Gradient cyan -> magenta
				const c = cyan.clone().lerp(magenta, t);
				cArr[i3] = c.r;
				cArr[i3 + 1] = c.g;
				cArr[i3 + 2] = c.b;
			}

			pos.needsUpdate = true;
			col.needsUpdate = true;
		}
	}

	dispose() {
		for (const arc of this.arcs) {
			arc.geometry.dispose();
			(arc.material as THREE.Material).dispose();
		}
	}
}
