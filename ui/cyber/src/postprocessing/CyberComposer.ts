import * as THREE from "three";
import { EffectComposer } from "three/examples/jsm/postprocessing/EffectComposer.js";
import { RenderPass } from "three/examples/jsm/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/examples/jsm/postprocessing/UnrealBloomPass.js";
import { ShaderPass } from "three/examples/jsm/postprocessing/ShaderPass.js";
import { GlitchPass } from "three/examples/jsm/postprocessing/GlitchPass.js";
import { ScanlineShader } from "./ScanlineShader.js";

export class CyberComposer {
	private composer: EffectComposer;
	private bloomPass: UnrealBloomPass;
	private glitchPass: GlitchPass;
	private scanlinePass: ShaderPass;

	constructor(renderer: THREE.WebGLRenderer, scene: THREE.Scene, camera: THREE.Camera) {
		this.composer = new EffectComposer(renderer);

		// 1. Render pass
		this.composer.addPass(new RenderPass(scene, camera));

		// 2. Bloom (half-res for perf)
		const res = new THREE.Vector2(window.innerWidth, window.innerHeight);
		this.bloomPass = new UnrealBloomPass(res.multiplyScalar(0.5), 1.2, 0.4, 0.3);
		this.composer.addPass(this.bloomPass);

		// 3. Scanlines + vignette + chromatic aberration
		this.scanlinePass = new ShaderPass(ScanlineShader);
		this.scanlinePass.uniforms.resolution.value = new THREE.Vector2(window.innerWidth, window.innerHeight);
		this.composer.addPass(this.scanlinePass);

		// 4. Glitch (disabled by default)
		this.glitchPass = new GlitchPass();
		this.glitchPass.enabled = false;
		this.composer.addPass(this.glitchPass);
	}

	setBloom(strength: number) {
		this.bloomPass.strength = strength;
	}

	setGlitch(enabled: boolean) {
		this.glitchPass.enabled = enabled;
	}

	/** Trigger a short glitch burst */
	glitchBurst(durationMs = 200) {
		this.glitchPass.enabled = true;
		setTimeout(() => {
			this.glitchPass.enabled = false;
		}, durationMs);
	}

	setSize(w: number, h: number) {
		this.composer.setSize(w, h);
		this.scanlinePass.uniforms.resolution.value.set(w, h);
	}

	render() {
		this.composer.render();
	}

	dispose() {
		this.composer.dispose();
	}
}
