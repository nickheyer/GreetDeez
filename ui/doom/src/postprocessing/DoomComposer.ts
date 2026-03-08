import * as THREE from "three";
import { EffectComposer } from "three/examples/jsm/postprocessing/EffectComposer.js";
import { RenderPass } from "three/examples/jsm/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/examples/jsm/postprocessing/UnrealBloomPass.js";
import { ShaderPass } from "three/examples/jsm/postprocessing/ShaderPass.js";
import { GlitchPass } from "three/examples/jsm/postprocessing/GlitchPass.js";
import { DoomShader } from "./DoomShader.js";

/** Objects on this layer contribute to bloom; objects only on layer 0 do not. */
export const BLOOM_LAYER = 1;

export class DoomComposer {
	private camera: THREE.Camera;

	// Selective bloom: renders only bloom-layer objects, applies bloom
	private bloomComposer: EffectComposer;
	private bloomPass: UnrealBloomPass;

	// Final: renders full scene, composites bloom, applies DoomShader + glitch
	private finalComposer: EffectComposer;
	private doomPass: ShaderPass;
	private glitchPass: GlitchPass;

	constructor(renderer: THREE.WebGLRenderer, scene: THREE.Scene, camera: THREE.Camera) {
		this.camera = camera;

		const res = new THREE.Vector2(window.innerWidth, window.innerHeight);

		// ── Bloom composer (off-screen) ──
		this.bloomComposer = new EffectComposer(renderer);
		this.bloomComposer.renderToScreen = false;
		this.bloomComposer.addPass(new RenderPass(scene, camera));
		this.bloomPass = new UnrealBloomPass(res.clone().multiplyScalar(0.5), 1.5, 0.6, 0.2);
		this.bloomComposer.addPass(this.bloomPass);

		// ── Final composer ──
		this.finalComposer = new EffectComposer(renderer);
		this.finalComposer.addPass(new RenderPass(scene, camera));

		// Blend bloom texture onto final render (additive)
		const blendPass = new ShaderPass(
			new THREE.ShaderMaterial({
				uniforms: {
					baseTexture: { value: null },
					bloomTexture: { value: this.bloomComposer.renderTarget2.texture },
				},
				vertexShader: /* glsl */ `
					varying vec2 vUv;
					void main() {
						vUv = uv;
						gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
					}
				`,
				fragmentShader: /* glsl */ `
					uniform sampler2D baseTexture;
					uniform sampler2D bloomTexture;
					varying vec2 vUv;
					void main() {
						gl_FragColor = texture2D(baseTexture, vUv) + texture2D(bloomTexture, vUv);
					}
				`,
			}),
			"baseTexture",
		);
		this.finalComposer.addPass(blendPass);

		// Film grain + scanlines + amber grading + vignette
		this.doomPass = new ShaderPass(DoomShader);
		this.doomPass.uniforms.resolution.value = new THREE.Vector2(window.innerWidth, window.innerHeight);
		this.finalComposer.addPass(this.doomPass);

		// Glitch (disabled by default)
		this.glitchPass = new GlitchPass();
		this.glitchPass.enabled = false;
		this.finalComposer.addPass(this.glitchPass);
	}

	setBloom(strength: number) {
		this.bloomPass.strength = strength;
	}

	setGlitch(enabled: boolean) {
		this.glitchPass.enabled = enabled;
	}

	glitchBurst(durationMs = 200) {
		this.glitchPass.enabled = true;
		setTimeout(() => {
			this.glitchPass.enabled = false;
		}, durationMs);
	}

	setSize(w: number, h: number) {
		this.bloomComposer.setSize(w, h);
		this.finalComposer.setSize(w, h);
		this.doomPass.uniforms.resolution.value.set(w, h);
	}

	update(time: number) {
		this.doomPass.uniforms.time.value = time;
	}

	render() {
		// 1. Bloom pass — camera only sees bloom-layer objects
		this.camera.layers.set(BLOOM_LAYER);
		this.bloomComposer.render();

		// 2. Final pass — camera sees all default-layer objects
		this.camera.layers.set(0);
		this.finalComposer.render();
	}

	dispose() {
		this.bloomComposer.dispose();
		this.finalComposer.dispose();
	}
}
