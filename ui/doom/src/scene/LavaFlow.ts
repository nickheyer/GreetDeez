import * as THREE from "three";

const lavaMaterial = new THREE.ShaderMaterial({
	uniforms: {
		uTime: { value: 0.0 },
	},
	vertexShader: /* glsl */ `
		varying vec2 vUv;
		void main() {
			vUv = uv;
			gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
		}
	`,
	fragmentShader: /* glsl */ `
		uniform float uTime;
		varying vec2 vUv;

		// Simple 2D noise
		float hash(vec2 p) {
			return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
		}

		float noise(vec2 p) {
			vec2 i = floor(p);
			vec2 f = fract(p);
			f = f * f * (3.0 - 2.0 * f);
			float a = hash(i);
			float b = hash(i + vec2(1.0, 0.0));
			float c = hash(i + vec2(0.0, 1.0));
			float d = hash(i + vec2(1.0, 1.0));
			return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
		}

		void main() {
			vec2 uv = vUv * 4.0;

			// 2-octave scrolling noise
			float n = noise(uv + uTime * 0.15) * 0.6;
			n += noise(uv * 2.0 - uTime * 0.1) * 0.4;

			// Interpolate between orange-red and amber-yellow
			vec3 hot = vec3(1.0, 0.27, 0.0);    // #ff4400
			vec3 warm = vec3(1.0, 0.67, 0.0);   // #ffaa00
			vec3 col = mix(hot, warm, n);

			// Boost brightness
			col *= 1.2;

			gl_FragColor = vec4(col, 0.9);
		}
	`,
	transparent: true,
	side: THREE.DoubleSide,
});

export class LavaFlow {
	readonly mesh: THREE.Mesh;
	readonly light: THREE.PointLight;

	constructor() {
		const geo = new THREE.PlaneGeometry(4, 30);
		geo.rotateX(-Math.PI / 2);
		this.mesh = new THREE.Mesh(geo, lavaMaterial.clone());
		this.mesh.position.set(0, -0.5, -14);

		// Warm orange glow that pulses — local to lava mesh
		this.light = new THREE.PointLight(0xff6600, 2.0, 20);
		this.light.position.set(0, 0.7, 0);
		this.mesh.add(this.light);
	}

	update(time: number) {
		(this.mesh.material as THREE.ShaderMaterial).uniforms.uTime.value = time;
		// Pulse the light intensity
		this.light.intensity = 1.5 + Math.sin(time * 2.0) * 0.5;
	}

	dispose() {
		this.mesh.geometry.dispose();
		(this.mesh.material as THREE.Material).dispose();
	}
}
