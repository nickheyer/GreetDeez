/** Combined scanlines + vignette + chromatic aberration in a single pass */
export const ScanlineShader = {
	uniforms: {
		tDiffuse: { value: null },
		resolution: { value: null },
		time: { value: 0.0 },
	},

	vertexShader: /* glsl */ `
		varying vec2 vUv;
		void main() {
			vUv = uv;
			gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
		}
	`,

	fragmentShader: /* glsl */ `
		uniform sampler2D tDiffuse;
		uniform vec2 resolution;
		uniform float time;
		varying vec2 vUv;

		void main() {
			// Chromatic aberration
			float aberr = 0.002;
			vec2 offset = vec2(aberr, 0.0);
			float r = texture2D(tDiffuse, vUv + offset).r;
			float g = texture2D(tDiffuse, vUv).g;
			float b = texture2D(tDiffuse, vUv - offset).b;
			vec3 color = vec3(r, g, b);

			// Scanlines
			float scanline = sin(vUv.y * resolution.y * 1.5) * 0.04;
			color -= scanline;

			// Vignette
			vec2 center = vUv - 0.5;
			float dist = length(center);
			float vignette = 1.0 - dist * dist * 1.2;
			color *= vignette;

			gl_FragColor = vec4(color, 1.0);
		}
	`,
};
