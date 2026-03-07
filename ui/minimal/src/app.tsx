import { useState, useEffect, useRef, useCallback } from "preact/hooks";
import { createGreeterServiceClient } from "@greetdeez/proto";
import "./app.css";

const client = createGreeterServiceClient();

interface Session {
	name: string;
	cmd: string[];
	type: number;
	desktop: string;
}

interface PowerCaps {
	canPoweroff: boolean;
	canReboot: boolean;
	canSuspend: boolean;
}

export function App() {
	const [hostname, setHostname] = useState("");
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [sessions, setSessions] = useState<Session[]>([]);
	const [selectedSession, setSelectedSession] = useState(0);
	const [error, setError] = useState("");
	const [capsLock, setCapsLock] = useState(false);
	const [clock, setClock] = useState("");
	const [powerCaps, setPowerCaps] = useState<PowerCaps>({
		canPoweroff: false,
		canReboot: false,
		canSuspend: false,
	});
	const [busy, setBusy] = useState(false);
	const userRef = useRef<HTMLInputElement>(null);

	// Clock
	useEffect(() => {
		const tick = () => {
			const now = new Date();
			setClock(
				now.toLocaleTimeString([], {
					hour: "2-digit",
					minute: "2-digit",
					second: "2-digit",
					hour12: false,
				}),
			);
		};
		tick();
		const id = setInterval(tick, 1000);
		return () => clearInterval(id);
	}, []);

	// Init
	useEffect(() => {
		(async () => {
			try {
				const [sessResp, infoResp, powerResp, stateResp] =
					await Promise.all([
						client.listSessions({}),
						client.getSystemInfo({}),
						client.getPowerCapabilities({}),
						client.getState({}),
					]);

				setSessions(sessResp.sessions ?? []);
				setHostname(infoResp.info?.hostname ?? "");

				if (powerResp.capabilities) {
					setPowerCaps(powerResp.capabilities);
				}

				const state = stateResp.state;
				if (state?.lastUser) setUsername(state.lastUser);
				if (state?.lastSession && sessResp.sessions) {
					const idx = sessResp.sessions.findIndex(
						(s: Session) => s.name === state.lastSession,
					);
					if (idx >= 0) setSelectedSession(idx);
				}
			} catch (e) {
				setError(String(e));
			}
		})();
	}, []);

	// Focus username on load
	useEffect(() => {
		userRef.current?.focus();
	}, []);

	const handleKeyDetect = useCallback(
		(e: KeyboardEvent) => {
			setCapsLock(e.getModifierState("CapsLock"));
		},
		[],
	);

	const handleLogin = async () => {
		if (busy || !username || !sessions.length) return;
		setError("");
		setBusy(true);

		try {
			const session = sessions[selectedSession];

			const authResp = await client.authenticate({ username, password });
			if (!authResp.success) {
				setError(authResp.error || "Authentication failed");
				setBusy(false);
				return;
			}

			const startResp = await client.startSession({ session });
			if (!startResp.success) {
				setError(startResp.error || "Session start failed");
				setBusy(false);
				return;
			}

			await client.saveState({
				state: { lastUser: username, lastSession: session.name },
			});
		} catch (e) {
			setError(String(e));
			setBusy(false);
		}
	};

	const handlePower = async (action: number) => {
		try {
			await client.executePowerAction({ action });
		} catch (e) {
			setError(String(e));
		}
	};

	return (
		<div class="shell">
			<div class="clock">{clock}</div>

			<div class="login-box">
				{hostname && <div class="hostname">{hostname}</div>}

				<div class="field">
					<label>username:</label>
					<input
						ref={userRef}
						type="text"
						value={username}
						onInput={(e) =>
							setUsername((e.target as HTMLInputElement).value)
						}
						onKeyDown={(e) => {
							handleKeyDetect(e as unknown as KeyboardEvent);
							if (e.key === "Enter") {
								(
									e.currentTarget
										?.parentElement?.nextElementSibling
										?.querySelector("input") as HTMLElement
								)?.focus();
							}
						}}
						autocomplete="off"
						spellcheck={false}
					/>
				</div>

				<div class="field">
					<label>password:</label>
					<input
						type="password"
						value={password}
						onInput={(e) =>
							setPassword((e.target as HTMLInputElement).value)
						}
						onKeyDown={(e) => {
							handleKeyDetect(e as unknown as KeyboardEvent);
							if (e.key === "Enter") handleLogin();
						}}
					/>
				</div>

				{sessions.length > 1 && (
					<div class="field">
						<label>session:</label>
						<select
							value={selectedSession}
							onChange={(e) =>
								setSelectedSession(
									Number(
										(e.target as HTMLSelectElement).value,
									),
								)
							}
						>
							{sessions.map((s, i) => (
								<option key={s.name} value={i}>
									{s.name}
								</option>
							))}
						</select>
					</div>
				)}

				{capsLock && <div class="caps-warn">CAPS LOCK</div>}
				<div class="error">{error}</div>

				<div class="actions">
					<button onClick={handleLogin} disabled={busy}>
						{busy ? "[...]" : "[login]"}
					</button>
				</div>
			</div>

			{(powerCaps.canSuspend ||
				powerCaps.canReboot ||
				powerCaps.canPoweroff) && (
				<div class="power-bar">
					{powerCaps.canSuspend && (
						<button onClick={() => handlePower(3)}>
							[suspend]
						</button>
					)}
					{powerCaps.canReboot && (
						<button onClick={() => handlePower(2)}>
							[reboot]
						</button>
					)}
					{powerCaps.canPoweroff && (
						<button onClick={() => handlePower(1)}>
							[poweroff]
						</button>
					)}
				</div>
			)}
		</div>
	);
}
