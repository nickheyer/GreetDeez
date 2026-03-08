import * as THREE from "three";

const vertexShader = /* glsl */ `
varying vec2 vUv;
void main() {
	vUv = uv;
	gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}
`;

const fragmentShader = /* glsl */ `
uniform vec3 uBgColor;
uniform float uBgAlpha;
uniform vec3 uBorderColor;
uniform float uBorderAlpha;
uniform float uBorderWidth;
uniform vec2 uSize;
uniform float uTime;
varying vec2 vUv;

void main() {
	vec2 pixel = vUv * uSize;
	float dx = min(pixel.x, uSize.x - pixel.x);
	float dy = min(pixel.y, uSize.y - pixel.y);
	float dist = min(dx, dy);

	// Edge glow — stronger near border
	float edgeGlow = smoothstep(uBorderWidth * 2.0, 0.0, dist) * uBorderAlpha;
	vec3 col = mix(uBgColor, uBorderColor, edgeGlow * 0.3);
	float alpha = mix(uBgAlpha, min(uBgAlpha + 0.15, 1.0), edgeGlow);

	// CRT flicker
	col *= 0.97 + 0.03 * sin(uTime * 30.0);

	gl_FragColor = vec4(col, alpha);
}
`;

export class PanelMaterial extends THREE.ShaderMaterial {
	constructor(width: number, height: number) {
		super({
			vertexShader,
			fragmentShader,
			uniforms: {
				uBgColor: { value: new THREE.Color(0x0a0a05) },
				uBgAlpha: { value: 0.93 },
				uBorderColor: { value: new THREE.Color(0xffaa00) },
				uBorderAlpha: { value: 0.3 },
				uBorderWidth: { value: 0.02 },
				uSize: { value: new THREE.Vector2(width, height) },
				uTime: { value: 0.0 },
			},
			transparent: true,
			side: THREE.DoubleSide,
			depthWrite: false,
		});
	}

	get borderColor(): THREE.Color {
		return this.uniforms.uBorderColor.value;
	}

	set borderAlpha(v: number) {
		this.uniforms.uBorderAlpha.value = v;
	}

	update(time: number) {
		this.uniforms.uTime.value = time;
	}
}
