/** Film grain + heavy scanlines + amber color grading + heavy vignette */
export const DoomShader = {
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

		// Simple hash for film grain
		float hash(vec2 p) {
			return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
		}

		void main() {
			vec3 color = texture2D(tDiffuse, vUv).rgb;

			// Amber color grading — boost R+G, reduce B (monochrome CRT feel)
			color.r *= 1.1;
			color.g *= 1.0;
			color.b *= 0.6;

			// Heavy scanlines
			float scanline = sin(vUv.y * resolution.y * 2.0) * 0.08;
			color -= scanline;

			// Film grain
			float grain = hash(vUv * resolution + time * 100.0) * 0.06;
			color += grain - 0.03;

			// Heavy vignette (falloff 1.8)
			vec2 center = vUv - 0.5;
			float dist = length(center);
			float vignette = 1.0 - dist * dist * 1.0;
			color *= vignette;

			gl_FragColor = vec4(color, 1.0);
		}
	`,
};
