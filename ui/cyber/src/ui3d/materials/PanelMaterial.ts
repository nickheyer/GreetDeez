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

	gl_FragColor = vec4(col, alpha);
}
`;

export class PanelMaterial extends THREE.ShaderMaterial {
	constructor(width: number, height: number) {
		super({
			vertexShader,
			fragmentShader,
			uniforms: {
				uBgColor: { value: new THREE.Color(0x020208) },
				uBgAlpha: { value: 0.93 },
				uBorderColor: { value: new THREE.Color(0x00ffff) },
				uBorderAlpha: { value: 0.3 },
				uBorderWidth: { value: 0.02 },
				uSize: { value: new THREE.Vector2(width, height) },
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
}
