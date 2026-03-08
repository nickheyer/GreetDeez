import * as THREE from "three";

interface LightEntry {
	light: THREE.PointLight;
	indicator: THREE.Mesh;
	baseIntensity: number;
	flickerSpeed: number;
	flickerOffset: number;
	color: THREE.Color;
	zPos: number;
}

const LIGHT_COUNT = 10;
const LIGHT_SPACING = 2.8;
const LIGHT_RANGE = 18;

export class FlickerLights {
	readonly group: THREE.Group;
	private lights: LightEntry[] = [];
	private _redOverride = false;
	private _enabledCount = 0;

	constructor() {
		this.group = new THREE.Group();

		const indicatorGeo = new THREE.SphereGeometry(0.06, 6, 6);

		for (let i = 0; i < LIGHT_COUNT; i++) {
			const z = 0.5 - i * LIGHT_SPACING;
			const side = i % 2 === 0 ? -1.5 : 1.5;
			const isTorch = i % 3 === 0;

			const color = new THREE.Color(isTorch ? 0xff6600 : 0xffaa00);
			const intensity = isTorch ? 3.0 : 2.2;

			const light = new THREE.PointLight(color, intensity, LIGHT_RANGE);
			light.position.set(side, 3.2, z);
			light.castShadow = false;

			const indicatorMat = new THREE.MeshBasicMaterial({
				color: color,
				transparent: true,
				opacity: 0.8,
			});
			const indicator = new THREE.Mesh(indicatorGeo, indicatorMat);
			indicator.position.copy(light.position);

			// Start all lights disabled — activated during boot
			light.visible = false;
			indicator.visible = false;

			this.group.add(light);
			this.group.add(indicator);

			this.lights.push({
				light,
				indicator,
				baseIntensity: intensity,
				flickerSpeed: 3.0 + Math.random() * 4.0,
				flickerOffset: Math.random() * Math.PI * 2,
				color,
				zPos: z,
			});
		}
	}

	get redOverride(): boolean {
		return this._redOverride;
	}

	set redOverride(v: boolean) {
		this._redOverride = v;
		if (!v) {
			// Restore original colors
			for (const entry of this.lights) {
				entry.light.color.copy(entry.color);
				(entry.indicator.material as THREE.MeshBasicMaterial).color.copy(entry.color);
			}
		}
	}

	/** Enable lights sequentially — called during boot as camera walks toward -Z */
	enableUpTo(cameraZ: number) {
		for (const entry of this.lights) {
			// Activate lights up to 8 units ahead of the camera
			const shouldEnable = entry.zPos > cameraZ - 8;
			entry.light.visible = shouldEnable;
			entry.indicator.visible = shouldEnable;
		}
	}

	/** Enable all lights */
	enableAll() {
		for (const entry of this.lights) {
			entry.light.visible = true;
			entry.indicator.visible = true;
		}
	}

	update(time: number) {
		for (const entry of this.lights) {
			if (!entry.light.visible) continue;

			// Layered sin flicker
			const flicker =
				Math.sin(time * entry.flickerSpeed + entry.flickerOffset) * 0.3 +
				Math.sin(time * entry.flickerSpeed * 2.7 + entry.flickerOffset * 1.3) * 0.15 +
				Math.sin(time * entry.flickerSpeed * 0.5 + entry.flickerOffset * 0.7) * 0.1;

			entry.light.intensity = entry.baseIntensity * (0.7 + flicker * 0.5);

			if (this._redOverride) {
				entry.light.color.set(0xff0000);
				(entry.indicator.material as THREE.MeshBasicMaterial).color.set(0xff0000);
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
