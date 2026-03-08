import "./styles/doom.css";
import { getClient } from "./client.js";
import { DoomScene } from "./scene/DoomScene.js";
import { SceneInteraction } from "./ui3d/SceneInteraction.js";
import { TerminalScreen } from "./ui3d/TerminalScreen.js";
import { PowerBar3D } from "./ui3d/PowerBar3D.js";
import { runBootSequence } from "./animations/BootSequence.js";
import { runLoginSequence } from "./animations/LoginSequence.js";
import { runErrorSequence } from "./animations/ErrorSequence.js";

async function main() {
	const canvas = document.getElementById("scene") as HTMLCanvasElement;

	const scene = new DoomScene(canvas);
	scene.start();

	const interaction = new SceneInteraction(canvas, scene.camera);

	const termZ = scene.terminal.screenCenter.z;

	// Terminal screen (positioned at terminal screen center)
	const terminal = new TerminalScreen({
		onSuccess: () => runLoginSequence(scene),
		onError: (_msg) => runErrorSequence(scene),
		onBoot: () => {},
	});
	terminal.position.copy(scene.terminal.screenCenter);
	scene.scene.add(terminal);

	// Power bar (below terminal, near keyboard shelf)
	const powerBar = new PowerBar3D(interaction, (msg) => {
		terminal.showError(msg);
	});
	powerBar.position.set(-0.5, 0.5, termZ);
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

		terminal.setSessions(sessResp.sessions ?? []);
		terminal.setHostname(infoResp.info?.hostname ?? "");

		if (powerResp.capabilities) {
			powerBar.setCaps(powerResp.capabilities);
		}

		const state = stateResp.state;
		if (state?.lastUser) terminal.setUsername(state.lastUser);
		if (state?.lastSession && sessResp.sessions) {
			const idx = sessResp.sessions.findIndex(
				(s: { name: string }) => s.name === state.lastSession,
			);
			if (idx >= 0) terminal.setSelectedSession(idx);
		}
	} catch (e) {
		console.error("Failed to fetch initial data:", e);
	}

	// Boot sequence (corridor walk-in + terminal boot)
	await runBootSequence(scene, terminal);

	// Idle: occasional rumble (brief glitch every 20-60s)
	const idleGlitch = () => {
		const delay = 20000 + Math.random() * 40000;
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
