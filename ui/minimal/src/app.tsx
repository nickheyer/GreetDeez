import { useState, useEffect, useRef, useCallback } from "preact/hooks";
import { createGreeterServiceClient } from "@nickheyer/greetdeez";
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
	const [dropOpen, setDropOpen] = useState(false);
	const [spinner, setSpinner] = useState(0);
	const userRef = useRef<HTMLInputElement>(null);

	const spinFrames = ["|", "/", "-", "\\"];

	useEffect(() => {
		if (!busy) return;
		const id = setInterval(() => setSpinner((s) => (s + 1) % 4), 120);
		return () => clearInterval(id);
	}, [busy]);

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

			// login does auth start and state in one rpc
			const resp = await client.login({ username, password, session });
			if (!resp.success) {
				const msgs = resp.messages?.filter(Boolean) ?? [];
				setError(msgs.length ? msgs.join(" ") : resp.error || "Authentication failed");
				setBusy(false);
			}
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
						<div class="session-picker">
							<button
								type="button"
								class="session-trigger"
								onClick={() => setDropOpen(!dropOpen)}
							>
								{sessions[selectedSession]?.name ?? "---"}
							</button>
							{dropOpen && (
								<>
									<div
										class="session-backdrop"
										onClick={() => setDropOpen(false)}
									/>
									<div class="session-menu">
										{sessions.map((s, i) => (
											<button
												key={s.name}
												type="button"
												class={`session-item${i === selectedSession ? " active" : ""}`}
												onClick={() => {
													setSelectedSession(i);
													setDropOpen(false);
												}}
											>
												{s.name}
											</button>
										))}
									</div>
								</>
							)}
						</div>
					</div>
				)}

				{capsLock && <div class="caps-warn">CAPS LOCK</div>}
				<div class="error">{error}</div>

				<div class="actions">
					<button onClick={handleLogin} disabled={busy}>
						{busy ? `[${spinFrames[spinner]}]` : "[login]"}
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
