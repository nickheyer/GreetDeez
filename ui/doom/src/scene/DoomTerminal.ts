import * as THREE from "three";

const crtVertexShader = /* glsl */ `
varying vec2 vUv;
void main() {
	vUv = uv;
	gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}
`;

const crtFragmentShader = /* glsl */ `
uniform float uTime;
varying vec2 vUv;

void main() {
	// CRT barrel distortion
	vec2 uv = vUv - 0.5;
	float r2 = dot(uv, uv);
	uv *= 1.0 + r2 * 0.15;
	uv += 0.5;

	// Out-of-bounds = black bezel
	if (uv.x < 0.0 || uv.x > 1.0 || uv.y < 0.0 || uv.y > 1.0) {
		gl_FragColor = vec4(0.0, 0.0, 0.0, 1.0);
		return;
	}

	// Dark amber phosphor base (visible but dark - like a powered-on CRT)
	vec3 col = vec3(0.03, 0.025, 0.008);

	// Scanlines — horizontal bands
	float scanline = sin(uv.y * 300.0) * 0.5 + 0.5;
	scanline = scanline * 0.12 + 0.88;
	col *= scanline;

	// Phosphor flicker
	col *= 0.96 + 0.04 * sin(uTime * 30.0);

	// Edge darkening (CRT curvature shadow)
	vec2 center = uv - 0.5;
	float dist = length(center);
	col *= 1.0 - dist * dist * 0.6;

	// Faint phosphor grid pattern
	float grid = sin(uv.x * 400.0) * 0.5 + 0.5;
	col *= grid * 0.05 + 0.95;

	gl_FragColor = vec4(col, 1.0);
}
`;

/** Boxy UAC computer terminal at the end of the corridor */
export class DoomTerminal {
	readonly group: THREE.Group;
	readonly screenCenter: THREE.Vector3;
	readonly screenMesh: THREE.Mesh;
	private screenMat: THREE.ShaderMaterial;

	constructor() {
		this.group = new THREE.Group();

		const bodyMat = new THREE.MeshStandardMaterial({
			color: 0x2a2a2a,
			roughness: 0.8,
			metalness: 0.2,
			emissive: 0x0a0a0a,
			emissiveIntensity: 0.3,
		});
		const bezelMat = new THREE.MeshStandardMaterial({
			color: 0x3a3a3a,
			roughness: 0.6,
			metalness: 0.3,
			emissive: 0x1a1008,
			emissiveIntensity: 0.5,
		});

		// Main body / pedestal
		const body = new THREE.Mesh(
			new THREE.BoxGeometry(1.6, 2.4, 0.8),
			bodyMat,
		);
		body.position.set(0, 1.2, 0);
		this.group.add(body);

		// Screen bezel — thicker frame around screen
		const bezel = new THREE.Mesh(
			new THREE.BoxGeometry(1.5, 1.5, 0.15),
			bezelMat,
		);
		bezel.position.set(0, 1.8, 0.42);
		this.group.add(bezel);

		// Inner bezel lip (recessed frame giving depth)
		const innerBezel = new THREE.Mesh(
			new THREE.BoxGeometry(1.3, 1.2, 0.06),
			new THREE.MeshStandardMaterial({
				color: 0x1a1a1a,
				roughness: 0.9,
				metalness: 0.1,
			}),
		);
		innerBezel.position.set(0, 1.8, 0.48);
		this.group.add(innerBezel);

		// Screen face — CRT shader
		this.screenMat = new THREE.ShaderMaterial({
			vertexShader: crtVertexShader,
			fragmentShader: crtFragmentShader,
			uniforms: {
				uTime: { value: 0.0 },
			},
		});
		this.screenMesh = new THREE.Mesh(
			new THREE.PlaneGeometry(1.3, 1.2),
			this.screenMat,
		);
		this.screenMesh.position.set(0, 1.8, 0.51);
		this.group.add(this.screenMesh);

		// CRT glow plane behind screen (emissive amber, additive)
		const glowMat = new THREE.MeshBasicMaterial({
			color: 0xffaa00,
			transparent: true,
			opacity: 0.06,
			blending: THREE.AdditiveBlending,
			depthWrite: false,
		});
		const glow = new THREE.Mesh(
			new THREE.PlaneGeometry(1.6, 1.5),
			glowMat,
		);
		glow.position.set(0, 1.8, 0.40);
		this.group.add(glow);

		// Amber screen light — illuminates the player/corridor in front of terminal
		const screenLight = new THREE.PointLight(0xffaa00, 1.5, 8);
		screenLight.position.set(0, 1.8, 0.8);
		this.group.add(screenLight);

		// Keyboard shelf
		const shelf = new THREE.Mesh(
			new THREE.BoxGeometry(1.4, 0.06, 0.5),
			bodyMat,
		);
		shelf.position.set(0, 0.7, 0.6);
		this.group.add(shelf);

		// Base stand (wider foot)
		const base = new THREE.Mesh(
			new THREE.BoxGeometry(1.8, 0.15, 1.0),
			bodyMat,
		);
		base.position.set(0, 0.075, 0.1);
		this.group.add(base);

		// Indicator LEDs on bezel (left side, small spheres)
		const ledGeo = new THREE.SphereGeometry(0.02, 6, 6);
		const ledColors = [0xff0000, 0xffaa00, 0x00ff00];
		for (let i = 0; i < 3; i++) {
			const ledMat = new THREE.MeshBasicMaterial({
				color: ledColors[i],
				transparent: true,
				opacity: 0.8,
			});
			const led = new THREE.Mesh(ledGeo, ledMat);
			led.position.set(-0.70, 1.15 + i * 0.06, 0.50);
			this.group.add(led);
		}

		// Side vents (left and right of body)
		const ventMat = new THREE.MeshStandardMaterial({
			color: 0x1a1a1a,
			roughness: 0.9,
			metalness: 0.1,
		});
		for (const side of [-1, 1]) {
			for (let i = 0; i < 4; i++) {
				const vent = new THREE.Mesh(
					new THREE.BoxGeometry(0.02, 0.04, 0.4),
					ventMat,
				);
				vent.position.set(side * 0.81, 1.5 + i * 0.1, 0.2);
				this.group.add(vent);
			}
		}

		// Position terminal at the far end of the corridor
		this.group.position.set(0, 0, -27);

		// Screen center in world space for UI placement
		this.screenCenter = new THREE.Vector3(0, 1.8, -27 + 0.52);
	}

	update(time: number) {
		this.screenMat.uniforms.uTime.value = time;
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
