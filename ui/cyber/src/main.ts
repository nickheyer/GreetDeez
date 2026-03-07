import "./styles/cyber.css";
import { getClient } from "./client.js";
import { CyberScene } from "./scene/CyberScene.js";
import { SceneInteraction } from "./ui3d/SceneInteraction.js";
import { LoginPanel3D } from "./ui3d/LoginPanel3D.js";
import { Clock3D } from "./ui3d/Clock3D.js";
import { PowerBar3D } from "./ui3d/PowerBar3D.js";
import { runBootSequence } from "./animations/BootSequence.js";
import { runLoginSequence } from "./animations/LoginSequence.js";
import { runErrorSequence } from "./animations/ErrorSequence.js";

async function main() {
	const canvas = document.getElementById("scene") as HTMLCanvasElement;

	const scene = new CyberScene(canvas);
	scene.start();

	const interaction = new SceneInteraction(canvas, scene.camera);

	// Clock (upper-right in world space)
	const clock = new Clock3D();
	clock.position.set(2.5, 3.5, 2.5);
	scene.scene.add(clock);

	// Login panel (centered)
	const panel = new LoginPanel3D(interaction, {
		onSuccess: () => runLoginSequence(scene),
		onError: () => runErrorSequence(scene),
		onBoot: () => {},
	});
	panel.position.set(0, 2.0, 2.5);
	scene.scene.add(panel);

	// Power bar (lower-right)
	const powerBar = new PowerBar3D(interaction, (msg) => {
		console.error("Power action failed:", msg);
	});
	powerBar.position.set(2.0, 0.5, 2.5);
	scene.scene.add(powerBar);

	// Fetch initial data from backend
	const client = getClient();
	try {
		const [sessResp, infoResp, powerResp, stateResp] = await Promise.all([
			client.listSessions({}),
			client.getSystemInfo({}),
			client.getPowerCapabilities({}),
			client.getState({}),
		]);

		panel.setSessions(sessResp.sessions ?? []);
		panel.setHostname(infoResp.info?.hostname ?? "");

		if (powerResp.capabilities) {
			powerBar.setCaps(powerResp.capabilities);
		}

		const state = stateResp.state;
		if (state?.lastUser) panel.setUsername(state.lastUser);
		if (state?.lastSession && sessResp.sessions) {
			const idx = sessResp.sessions.findIndex(
				(s: { name: string }) => s.name === state.lastSession,
			);
			if (idx >= 0) panel.setSelectedSession(idx);
		}
	} catch (e) {
		console.error("Failed to fetch initial data:", e);
	}

	// Boot sequence (scene fade-in + panel typing animation)
	await runBootSequence(scene, panel);

	// Idle glitch pulse every 30-60s
	const idleGlitch = () => {
		const delay = 30000 + Math.random() * 30000;
		setTimeout(() => {
			scene.glitchEnabled = true;
			setTimeout(() => {
				scene.glitchEnabled = false;
			}, 150);
			idleGlitch();
		}, delay);
	};
	idleGlitch();
}

main();
