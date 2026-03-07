import "./styles/cyber.css";
import { getClient } from "./client.js";
import { CyberScene } from "./scene/CyberScene.js";
import { LoginOverlay } from "./ui/LoginOverlay.js";
import { PowerBar } from "./ui/PowerBar.js";
import { Clock } from "./ui/Clock.js";
import { runBootSequence } from "./animations/BootSequence.js";
import { runLoginSequence } from "./animations/LoginSequence.js";
import { runErrorSequence } from "./animations/ErrorSequence.js";

async function main() {
	const canvas = document.getElementById("scene") as HTMLCanvasElement;
	const ui = document.getElementById("ui")!;

	const scene = new CyberScene(canvas);
	scene.start();

	// Clock
	new Clock(ui);

	// Login overlay
	const overlay = new LoginOverlay(ui, {
		onSuccess: () => runLoginSequence(scene),
		onError: () => runErrorSequence(scene),
		onBoot: () => {},
	});

	// Power bar
	const powerBar = new PowerBar(ui, (msg) => {
		// Power action errors shown via console (no notification overlay for power)
		console.error("Power action failed:", msg);
	});

	// Fetch initial data from backend
	const client = getClient();
	try {
		const [sessResp, infoResp, powerResp, stateResp] = await Promise.all([
			client.listSessions({}),
			client.getSystemInfo({}),
			client.getPowerCapabilities({}),
			client.getState({}),
		]);

		overlay.setSessions(sessResp.sessions ?? []);
		overlay.setHostname(infoResp.info?.hostname ?? "");

		if (powerResp.capabilities) {
			powerBar.setCaps(powerResp.capabilities);
		}

		const state = stateResp.state;
		if (state?.lastUser) overlay.setUsername(state.lastUser);
		if (state?.lastSession && sessResp.sessions) {
			const idx = sessResp.sessions.findIndex(
				(s: { name: string }) => s.name === state.lastSession,
			);
			if (idx >= 0) overlay.setSelectedSession(idx);
		}
	} catch (e) {
		console.error("Failed to fetch initial data:", e);
	}

	// Boot sequence (scene fade-in + overlay typing animation)
	await runBootSequence(scene, overlay);

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
