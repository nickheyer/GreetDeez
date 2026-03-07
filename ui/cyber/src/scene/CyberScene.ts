import * as THREE from "three";
import { GridFloor } from "./GridFloor.js";
import { ParticleRain } from "./ParticleRain.js";
import { NeonCity } from "./NeonCity.js";
import { HoloPanel } from "./HoloPanel.js";
import { DataStream } from "./DataStream.js";
import { CyberComposer } from "../postprocessing/CyberComposer.js";

export class CyberScene {
	readonly renderer: THREE.WebGLRenderer;
	readonly scene: THREE.Scene;
	readonly camera: THREE.PerspectiveCamera;

	private composer: CyberComposer;
	private grid: GridFloor;
	private particles: ParticleRain;
	private city: NeonCity;
	private holos: HoloPanel;
	private streams: DataStream;

	private clock = new THREE.Clock();
	private animId = 0;
	private basePos = new THREE.Vector3(0, 2.5, 8);

	// Used by animations to control bloom/glitch
	bloomStrength = 1.2;
	glitchEnabled = false;
	opacity = 0; // scene fades in during boot

	constructor(canvas: HTMLCanvasElement) {
		// Renderer
		this.renderer = new THREE.WebGLRenderer({
			canvas,
			antialias: false,
			powerPreference: "low-power",
		});
		this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 1.5));
		this.renderer.setSize(window.innerWidth, window.innerHeight);
		this.renderer.setClearColor(0x020208);

		// Scene
		this.scene = new THREE.Scene();
		this.scene.fog = new THREE.FogExp2(0x020208, 0.02);

		// Camera
		this.camera = new THREE.PerspectiveCamera(60, window.innerWidth / window.innerHeight, 0.1, 200);
		this.camera.position.copy(this.basePos);
		this.camera.lookAt(0, 1, 0);

		// Scene elements
		this.grid = new GridFloor();
		this.scene.add(this.grid.mesh);

		this.particles = new ParticleRain();
		this.scene.add(this.particles.points);

		this.city = new NeonCity();
		this.scene.add(this.city.group);

		this.holos = new HoloPanel();
		this.scene.add(this.holos.group);

		this.streams = new DataStream();
		this.scene.add(this.streams.group);

		// Post-processing
		this.composer = new CyberComposer(this.renderer, this.scene, this.camera);

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

		// Idle camera drift (Lissajous)
		this.camera.position.x = this.basePos.x + Math.sin(elapsed / 15 * Math.PI * 2) * 0.3;
		this.camera.position.y = this.basePos.y + Math.cos(elapsed / 11 * Math.PI * 2) * 0.15;
		this.camera.lookAt(0, 1, 0);

		// Update elements
		this.grid.update(elapsed);
		this.particles.update(dt);
		this.holos.update(elapsed);
		this.streams.update(elapsed);

		// Apply dynamic composer settings
		this.composer.setBloom(this.bloomStrength);
		this.composer.setGlitch(this.glitchEnabled);

		// Render with opacity fade
		this.renderer.domElement.style.opacity = String(this.opacity);
		this.composer.render();
	};

	/** Dolly camera forward for login success animation */
	dollyForward(duration: number): Promise<void> {
		return new Promise((resolve) => {
			const start = this.basePos.z;
			const end = 2;
			const startTime = this.clock.elapsedTime;

			const step = () => {
				const t = Math.min((this.clock.elapsedTime - startTime) / duration, 1);
				const eased = t * t; // ease-in
				this.basePos.z = start + (end - start) * eased;

				if (t < 1) {
					requestAnimationFrame(step);
				} else {
					resolve();
				}
			};
			step();
		});
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
		this.grid.dispose();
		this.particles.dispose();
		this.city.dispose();
		this.holos.dispose();
		this.streams.dispose();
		this.composer.dispose();
		this.renderer.dispose();
	}
}
