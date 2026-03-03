<script lang="ts">
	import { client } from '$lib/client';
	import {
		AuthOutcome,
		AuthMessageType,
		SessionType,
		PowerAction,
		type Session,
		type PowerCapabilities,
	} from '@greetdeez/proto';
	import { onMount, tick } from 'svelte';
	import { LoaderCircle, Power, RotateCcw, Moon, TriangleAlert, ChevronDown } from '@lucide/svelte';

	let username = $state('');
	let password = $state('');
	let sessions = $state<Session[]>([]);
	let selectedName = $state('');
	let selectedType = $state<SessionType>(SessionType.UNSPECIFIED);
	let errorMsg = $state('');
	let pamMessages = $state<string[]>([]);
	let status = $state<'idle' | 'authenticating' | 'starting' | 'cooldown'>('idle');
	let hostname = $state('');
	let now = $state(new Date());
	let shaking = $state(false);
	let success = $state(false);
	let capsLock = $state(false);
	let debugMode = $state(false);
	let debugLogs = $state<string[]>([]);
	let powerCaps = $state<PowerCapabilities | null>(null);

	let passwordInput: HTMLInputElement | undefined = $state();
	let usernameInput: HTMLInputElement | undefined = $state();
	let logPane: HTMLDivElement | undefined = $state();
	let debugPanelHeight = $state(35);
	let dragging = $state(false);
	let nameDropOpen = $state(false);
	let typeDropOpen = $state(false);

	function onDragStart(e: PointerEvent) {
		dragging = true;
		const startY = e.clientY;
		const startH = debugPanelHeight;
		const vh = window.innerHeight;
		(e.target as HTMLElement).setPointerCapture(e.pointerId);

		function onMove(ev: PointerEvent) {
			const delta = startY - ev.clientY;
			debugPanelHeight = Math.min(80, Math.max(10, startH + (delta / vh) * 100));
		}
		function onUp() {
			dragging = false;
			window.removeEventListener('pointermove', onMove);
			window.removeEventListener('pointerup', onUp);
		}
		window.addEventListener('pointermove', onMove);
		window.addEventListener('pointerup', onUp);
	}

	function sessionTypeLabel(t: SessionType): string {
		switch (t) {
			case SessionType.WAYLAND: return 'wayland';
			case SessionType.X11: return 'x11';
			default: return 'unknown';
		}
	}

	let uniqueNames = $derived([...new Set(sessions.map((s) => s.name))]);
	let availableTypes = $derived(
		sessions.filter((s) => s.name === selectedName).map((s) => s.type)
	);
	let selectedSession = $derived(
		sessions.find((s) => s.name === selectedName && s.type === selectedType)
	);
	let time = $derived(now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
	let date = $derived(
		now.toLocaleDateString([], { weekday: 'long', month: 'long', day: 'numeric' })
	);
	let powerEnabled = $derived(
		powerCaps != null && (powerCaps.canPoweroff || powerCaps.canReboot || powerCaps.canSuspend)
	);
	let formDisabled = $derived(status !== 'idle');

	onMount(() => {
		debugMode = !window.__greetdeez_rpc__;

		async function init() {
			const sessResp = await client.listSessions();
			const s = sessResp.sessions;
			sessions = s;

			if (s.length > 0) {
				selectedName = s[0].name;
				selectedType = s[0].type;
			}

			const stateResp = await client.getState();
			const st = stateResp.state;
			if (st.lastUser) {
				username = st.lastUser;
			}
			if (st.lastSession && s.length > 0) {
				const match = s.find((sess: Session) => sess.name === st.lastSession);
				if (match) {
					selectedName = match.name;
					selectedType = match.type;
				}
			}
			if (username) {
				passwordInput?.focus();
			} else {
				usernameInput?.focus();
			}

			const sysResp = await client.getSystemInfo();
			hostname = sysResp.info.hostname;

			const powerResp = await client.getPowerCapabilities();
			powerCaps = powerResp.capabilities;
		}
		init();

		const timer = setInterval(() => (now = new Date()), 1000);
		return () => clearInterval(timer);
	});

	$effect(() => {
		if (!debugMode) return;
		const poll = async () => {
			const { lines } = await client.getLogs();
			debugLogs = lines;
			await tick();
			if (logPane) logPane.scrollTop = logPane.scrollHeight;
		};
		poll();
		const id = setInterval(poll, 1000);
		return () => clearInterval(id);
	});

	function handleKeydown(e: KeyboardEvent) {
		capsLock = e.getModifierState('CapsLock');
	}

	function handleUsernameKeydown(e: KeyboardEvent) {
		handleKeydown(e);
		if (e.key === 'Enter' && username) {
			e.preventDefault();
			passwordInput?.focus();
		}
	}

	async function handleLogin(e: SubmitEvent) {
		e.preventDefault();
		if (!username || !selectedSession) return;

		errorMsg = '';
		pamMessages = [];
		status = 'authenticating';

		try {
			// Step 1: Create session
			let resp = await client.createSession({ username });

			// Step 2: Handle auth message loop
			while (resp.outcome === AuthOutcome.AUTH_MESSAGE) {
				if (resp.authMessage?.type === AuthMessageType.SECRET) {
					// Wait for password to be entered
					if (!password) {
						// Focus password input and wait for submit
						status = 'idle';
						passwordInput?.focus();
						return;
					}
					const authResp = await client.postAuth({ response: password });
					resp = {
						outcome: authResp.outcome,
						authMessage: authResp.authMessage,
						success: authResp.success,
						failure: authResp.failure,
					};
				} else if (resp.authMessage?.type === AuthMessageType.VISIBLE) {
					// Visible prompt — use password field value as response
					if (!password) {
						status = 'idle';
						passwordInput?.focus();
						return;
					}
					const authResp = await client.postAuth({ response: password });
					resp = {
						outcome: authResp.outcome,
						authMessage: authResp.authMessage,
						success: authResp.success,
						failure: authResp.failure,
					};
				} else if (
					resp.authMessage?.type === AuthMessageType.INFO ||
					resp.authMessage?.type === AuthMessageType.ERROR
				) {
					// Info/error messages: display and acknowledge
					if (resp.authMessage.message) {
						pamMessages = [...pamMessages, resp.authMessage.message];
					}
					const authResp = await client.postAuth({});
					resp = {
						outcome: authResp.outcome,
						authMessage: authResp.authMessage,
						success: authResp.success,
						failure: authResp.failure,
					};
				} else {
					// Unknown message type, acknowledge
					const authResp = await client.postAuth({});
					resp = {
						outcome: authResp.outcome,
						authMessage: authResp.authMessage,
						success: authResp.success,
						failure: authResp.failure,
					};
				}
			}

			// Step 3: Handle outcome
			if (resp.outcome === AuthOutcome.FAILURE) {
				errorMsg = resp.failure?.description || 'Authentication failed';
				status = 'cooldown';
				password = '';
				shaking = true;
				setTimeout(() => (shaking = false), 500);
				setTimeout(() => (errorMsg = ''), 4000);
				setTimeout(() => {
					status = 'idle';
					passwordInput?.focus();
				}, 2000);
				return;
			}

			if (resp.outcome !== AuthOutcome.SUCCESS) {
				errorMsg = 'Unexpected auth response';
				status = 'idle';
				return;
			}

			// Step 4: Start session
			status = 'starting';

			const startResult = await client.startSession({
				cmd: [...selectedSession.cmd],
				type: selectedSession.type,
				desktop: selectedSession.desktop,
			});
			if (!startResult.ok) {
				errorMsg = startResult.error || 'Failed to start session';
				status = 'idle';
				shaking = true;
				setTimeout(() => (shaking = false), 500);
				return;
			}

			// Save state for next login
			client.saveState({
				state: { lastUser: username, lastSession: selectedSession.name }
			});

			success = true;
		} catch (err) {
			errorMsg = err instanceof Error ? err.message : 'Login failed';
			status = 'cooldown';
			password = '';
			shaking = true;
			setTimeout(() => (shaking = false), 500);
			setTimeout(() => (errorMsg = ''), 4000);
			setTimeout(() => {
				status = 'idle';
				passwordInput?.focus();
			}, 2000);
		}
	}

	async function handlePower(action: PowerAction) {
		await client.executePowerAction({ action });
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div
	class="flex w-full flex-col items-center justify-center gap-12"
	class:animate-success-fade={success}
	style:height={debugMode ? `${100 - debugPanelHeight}vh` : '100%'}
>
	<div class="animate-fade-up flex flex-col items-center gap-1 delay-100">
		<span class="clock-glow text-7xl font-extralight tracking-wide">
			{time}
		</span>
		<span class="text-base text-text-muted">{date}</span>
	</div>

	<form
		onsubmit={handleLogin}
		class="login-panel animate-fade-up flex w-95 flex-col items-center gap-4 rounded-2xl border border-white/8 p-8 delay-300"
		class:animate-shake={shaking}
	>
		<p class="mb-2 text-xs font-medium tracking-[0.2em] uppercase text-text-muted">
			{hostname}
		</p>

		<input
			bind:this={usernameInput}
			type="text"
			placeholder="Username"
			bind:value={username}
			onkeydown={handleUsernameKeydown}
			disabled={formDisabled}
			autocomplete="off"
			class="input-field"
		/>

		<div class="relative w-full">
			<input
				bind:this={passwordInput}
				type="password"
				placeholder="Password"
				bind:value={password}
				onkeydown={handleKeydown}
				disabled={formDisabled}
				class="input-field w-full"
			/>
			{#if capsLock}
				<div class="absolute top-1/2 right-3 flex -translate-y-1/2 items-center gap-1 text-warning">
					<TriangleAlert size={14} />
					<span class="text-[10px] font-medium uppercase tracking-wider">Caps Lock</span>
				</div>
			{/if}
		</div>

		{#if errorMsg}
			<p class="animate-slide-in text-sm text-error">{errorMsg}</p>
		{/if}

		{#if pamMessages.length > 0}
			<div class="w-full rounded-lg border border-white/10 bg-white/5 px-3 py-2">
				{#each pamMessages as msg}
					<p class="animate-slide-in text-xs text-text-muted">{msg}</p>
				{/each}
			</div>
		{/if}

		<button
			type="submit"
			disabled={formDisabled}
			class="flex w-full items-center justify-center gap-2 rounded-lg bg-white/10 py-3 text-sm font-medium text-text transition-all duration-200 hover:bg-white/15 disabled:cursor-not-allowed disabled:opacity-40"
		>
			{#if status === 'authenticating'}
				<LoaderCircle size={16} class="animate-spin-slow" />
				Authenticating...
			{:else if status === 'starting'}
				<LoaderCircle size={16} class="animate-spin-slow" />
				Starting session...
			{:else if status === 'cooldown'}
				<LoaderCircle size={16} class="animate-spin-slow" />
				Please wait...
			{:else}
				Log in
			{/if}
		</button>

		{#if sessions.length > 0}
			<div class="flex w-full items-center justify-center gap-2 pt-1">
				{#if uniqueNames.length === 1}
					<span class="session-badge session-badge-name">{selectedName}</span>
				{:else}
					<div class="session-dropdown">
						<button
							type="button"
							class="session-dropdown-trigger"
							onclick={() => { nameDropOpen = !nameDropOpen; typeDropOpen = false; }}
						>
							{selectedName}
							<ChevronDown size={12} class="session-dropdown-chevron" />
						</button>
						{#if nameDropOpen}
							<div class="session-dropdown-backdrop" onclick={() => (nameDropOpen = false)}></div>
							<div class="session-dropdown-menu">
								{#each uniqueNames as name}
									<button
										type="button"
										class="session-dropdown-item"
										class:session-dropdown-item-active={name === selectedName}
										onclick={() => {
											selectedName = name;
											const types = sessions.filter((s) => s.name === name).map((s) => s.type);
											if (!types.includes(selectedType)) selectedType = types[0];
											nameDropOpen = false;
										}}
									>{name}</button>
								{/each}
							</div>
						{/if}
					</div>
				{/if}

				{#if availableTypes.length === 1}
					<span
						class="session-badge"
						class:session-badge-wayland={selectedType === SessionType.WAYLAND}
						class:session-badge-x11={selectedType !== SessionType.WAYLAND}
					>{sessionTypeLabel(selectedType)}</span>
				{:else if availableTypes.length > 1}
					<div class="session-dropdown">
						<button
							type="button"
							class="session-dropdown-trigger"
							onclick={() => { typeDropOpen = !typeDropOpen; nameDropOpen = false; }}
						>
							{sessionTypeLabel(selectedType)}
							<ChevronDown size={12} class="session-dropdown-chevron" />
						</button>
						{#if typeDropOpen}
							<div class="session-dropdown-backdrop" onclick={() => (typeDropOpen = false)}></div>
							<div class="session-dropdown-menu">
								{#each availableTypes as t}
									<button
										type="button"
										class="session-dropdown-item"
										class:session-dropdown-item-active={t === selectedType}
										onclick={() => { selectedType = t; typeDropOpen = false; }}
									>{sessionTypeLabel(t)}</button>
								{/each}
							</div>
						{/if}
					</div>
				{/if}
			</div>
		{/if}
	</form>
</div>

{#if powerEnabled}
	<div class="fixed right-6 flex items-center gap-1" style:bottom={debugMode ? `calc(${debugPanelHeight}vh + 1.5rem)` : '1.5rem'}>
		{#if powerCaps?.canSuspend}
			<button
				onclick={() => handlePower(PowerAction.SUSPEND)}
				class="power-btn"
				title="Suspend"
			>
				<Moon size={18} />
			</button>
		{/if}
		{#if powerCaps?.canReboot}
			<button
				onclick={() => handlePower(PowerAction.REBOOT)}
				class="power-btn"
				title="Reboot"
			>
				<RotateCcw size={18} />
			</button>
		{/if}
		{#if powerCaps?.canPoweroff}
			<button
				onclick={() => handlePower(PowerAction.POWEROFF)}
				class="power-btn"
				title="Power Off"
			>
				<Power size={18} />
			</button>
		{/if}
	</div>
{/if}

{#if debugMode}
	<div class="debug-panel" style:height={`${debugPanelHeight}vh`}>
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="debug-drag-handle" class:debug-drag-active={dragging} onpointerdown={onDragStart}>
			<div class="debug-drag-grip"></div>
		</div>
		<div class="debug-panel-body">
			<div class="debug-pane">
				<div class="debug-pane-header">Sessions</div>
				<pre class="debug-pane-content">{JSON.stringify(sessions, null, 2)}</pre>
			</div>
			<div class="debug-pane debug-pane-border">
				<div class="debug-pane-header">Logs ({debugLogs.length})</div>
				<div class="debug-pane-content" bind:this={logPane}>
					{#each debugLogs as line}
						<div class="debug-log-line">{line}</div>
					{/each}
				</div>
			</div>
		</div>
	</div>
{/if}
