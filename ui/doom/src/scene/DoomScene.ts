import * as THREE from "three";
import { DoomCorridor } from "./DoomCorridor.js";
import { LavaFlow } from "./LavaFlow.js";
import { FlickerLights } from "./FlickerLights.js";
import { DoomTerminal } from "./DoomTerminal.js";
import { HellPortal } from "./HellPortal.js";
import { KeyboardWeapon } from "./KeyboardWeapon.js";
import { DoomComposer, BLOOM_LAYER } from "../postprocessing/DoomComposer.js";

export class DoomScene {
	readonly renderer: THREE.WebGLRenderer;
	readonly scene: THREE.Scene;
	readonly camera: THREE.PerspectiveCamera;

	private composer: DoomComposer;
	readonly corridor: DoomCorridor;
	readonly lava: LavaFlow;
	readonly lights: FlickerLights;
	readonly terminal: DoomTerminal;
	readonly portal: HellPortal;
	readonly weapon: KeyboardWeapon;

	private clock = new THREE.Clock();
	private animId = 0;
	private basePos = new THREE.Vector3(0, 1.7, 1);

	// Used by animations
	bloomStrength = 1.5;
	glitchEnabled = false;
	opacity = 0;
	swayAmplitude = 1.0;

	constructor(canvas: HTMLCanvasElement) {
		// Renderer
		this.renderer = new THREE.WebGLRenderer({
			canvas,
			antialias: false,
			powerPreference: "low-power",
		});
		this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 1.5));
		this.renderer.setSize(window.innerWidth, window.innerHeight);
		this.renderer.setClearColor(0x0a0503);

		// Scene
		this.scene = new THREE.Scene();
		this.scene.fog = new THREE.FogExp2(0x0a0503, 0.008);

		// Camera — eye height, far back (corridor entrance)
		this.camera = new THREE.PerspectiveCamera(60, window.innerWidth / window.innerHeight, 0.1, 200);
		this.camera.position.copy(this.basePos);
		this.camera.lookAt(0, 1.7, -10);

		// Ambient light — bright enough that MeshStandardMaterial walls are visible
		const ambient = new THREE.AmbientLight(0x886655, 2.5);
		this.scene.add(ambient);

		// Scene elements
		this.corridor = new DoomCorridor();
		this.scene.add(this.corridor.group);

		this.lava = new LavaFlow();
		this.scene.add(this.lava.mesh);

		this.lights = new FlickerLights();
		this.scene.add(this.lights.group);

		this.terminal = new DoomTerminal();
		this.scene.add(this.terminal.group);

		this.portal = new HellPortal();
		this.scene.add(this.portal.group);

		// Keyboard weapon — attached to camera
		this.weapon = new KeyboardWeapon();
		this.camera.add(this.weapon.group);
		this.scene.add(this.camera);

		// Enable bloom on scene elements (UI stays on layer 0)
		const bloomRoots = [
			this.corridor.group,
			this.lava.mesh,
			this.lights.group,
			this.terminal.group,
			this.portal.group,
			this.weapon.group,
		];
		for (const root of bloomRoots) {
			root.traverse((child) => child.layers.enable(BLOOM_LAYER));
		}

		// Post-processing
		this.composer = new DoomComposer(this.renderer, this.scene, this.camera);

		// Events
		window.addEventListener("resize", this.onResize);
	}

	start() {
		this.clock.start();
		this.animate();
	}

	private animate = () => {
		this.animId = requestAnimationFrame(this.animate);

		const dt = Math.min(this.clock.getDelta(), 0.1);
		const elapsed = this.clock.elapsedTime;

		// Idle camera: subtle breathing bob + horizontal sway (configurable amplitude)
		const sway = this.swayAmplitude;
		this.camera.position.x = this.basePos.x + Math.sin(elapsed * 0.4) * 0.04 * sway;
		this.camera.position.y = this.basePos.y + Math.sin(elapsed * 0.6) * 0.02 * sway;
		this.camera.position.z = this.basePos.z;
		this.camera.lookAt(this.camera.position.x, 1.7, this.basePos.z - 10);

		// Update scene elements
		this.lava.update(elapsed);
		this.lights.update(elapsed);
		this.portal.update(elapsed, dt);
		this.weapon.update(elapsed);
		this.terminal.update(elapsed);

		// Composer settings
		this.composer.setBloom(this.bloomStrength);
		this.composer.setGlitch(this.glitchEnabled);
		this.composer.update(elapsed);

		// Render with opacity fade
		this.renderer.domElement.style.opacity = String(this.opacity);
		this.composer.render();
	};

	/** Dolly camera to target Z over duration */
	dollyTo(targetZ: number, duration: number): Promise<void> {
		return new Promise((resolve) => {
			const start = this.basePos.z;
			const startTime = this.clock.elapsedTime;

			const step = () => {
				const t = Math.min((this.clock.elapsedTime - startTime) / duration, 1);
				// ease-in-out
				const eased = t < 0.5 ? 2 * t * t : 1 - (-2 * t + 2) ** 2 / 2;
				this.basePos.z = start + (targetZ - start) * eased;

				if (t < 1) {
					requestAnimationFrame(step);
				} else {
					resolve();
				}
			};
			step();
		});
	}

	set lightsRedOverride(v: boolean) {
		this.lights.redOverride = v;
	}

	private onResize = () => {
		const w = window.innerWidth;
		const h = window.innerHeight;
		this.camera.aspect = w / h;
		this.camera.updateProjectionMatrix();
		this.renderer.setSize(w, h);
		this.composer.setSize(w, h);
	};

	dispose() {
		cancelAnimationFrame(this.animId);
		window.removeEventListener("resize", this.onResize);
		this.corridor.dispose();
		this.lava.dispose();
		this.lights.dispose();
		this.terminal.dispose();
		this.portal.dispose();
		this.weapon.dispose();
		this.composer.dispose();
		this.renderer.dispose();
	}
}
