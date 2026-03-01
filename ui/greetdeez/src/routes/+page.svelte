<script lang="ts">
	import { bridge, type Session, type AppConfig } from '$lib/bridge';
	import { onMount, tick } from 'svelte';
	import { LoaderCircle, Power, RotateCcw, Moon, TriangleAlert } from '@lucide/svelte';

	let username = $state('');
	let password = $state('');
	let sessions = $state<Session[]>([]);
	let selectedSessionIdx = $state('0');
	let errorMsg = $state('');
	let pamMessages = $state<string[]>([]);
	let status = $state<'idle' | 'authenticating' | 'starting' | 'cooldown'>('idle');
	let hostname = $state('');
	let now = $state(new Date());
	let shaking = $state(false);
	let success = $state(false);
	let cfg = $state<AppConfig | null>(null);
	let capsLock = $state(false);
	let debugLogs = $state<string[]>([]);

	let passwordInput: HTMLInputElement | undefined = $state();
	let usernameInput: HTMLInputElement | undefined = $state();
	let logPane: HTMLDivElement | undefined = $state();
	let debugPanelHeight = $state(35);
	let dragging = $state(false);

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

	let selectedSession = $derived(sessions[parseInt(selectedSessionIdx)]);
	let time = $derived(now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
	let date = $derived(
		now.toLocaleDateString([], { weekday: 'long', month: 'long', day: 'numeric' })
	);
	let powerEnabled = $derived(cfg?.power?.enabled ?? true);
	let formDisabled = $derived(status !== 'idle');

	onMount(() => {
		bridge.getSessions().then((s) => {
			sessions = s;

			// Restore last session after sessions are loaded
			bridge.getLastState().then((st) => {
				if (st.last_user) {
					username = st.last_user;
				}
				if (st.last_session && s.length > 0) {
					const idx = s.findIndex((sess) => sess.name === st.last_session);
					if (idx >= 0) {
						selectedSessionIdx = String(idx);
					}
				}
				// Focus the right field based on whether we restored a username
				if (username) {
					passwordInput?.focus();
				} else {
					usernameInput?.focus();
				}
			});
		});
		bridge.getHostname().then((h) => (hostname = h));
		bridge.getConfig().then((c) => {
			cfg = c;
			if (c.theme?.accent_color) {
				document.documentElement.style.setProperty('--color-accent', c.theme.accent_color);
			}
			if (c.theme?.aurora_speed !== undefined) {
				document.documentElement.style.setProperty(
					'--aurora-speed-mult',
					String(c.theme.aurora_speed)
				);
			}
		});

		const timer = setInterval(() => (now = new Date()), 1000);
		return () => clearInterval(timer);
	});

	$effect(() => {
		if (!cfg?.debug) return;
		const poll = async () => {
			debugLogs = await bridge.getLogs();
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
		if (!username || !password || !selectedSession) return;

		errorMsg = '';
		pamMessages = [];
		status = 'authenticating';

		const authResult = await bridge.login(username, password);
		if (!authResult.ok) {
			errorMsg = authResult.error ?? 'Authentication failed';
			status = 'cooldown';
			password = '';
			shaking = true;
			setTimeout(() => (shaking = false), 500);
			setTimeout(() => (errorMsg = ''), 4000);

			// Rate limit: 2s cooldown after failed auth
			setTimeout(() => {
				status = 'idle';
				passwordInput?.focus();
			}, 2000);
			return;
		}

		// Collect PAM info messages (MOTD, password expiry, etc.)
		if (authResult.messages && authResult.messages.length > 0) {
			pamMessages = authResult.messages;
		}

		status = 'starting';
		const startResult = await bridge.startSession(selectedSession);
		if (!startResult.ok) {
			errorMsg = startResult.error ?? 'Failed to start session';
			status = 'idle';
			shaking = true;
			setTimeout(() => (shaking = false), 500);
			return;
		}

		// Save state for next login
		bridge.saveState({
			last_user: username,
			last_session: selectedSession.name
		});

		success = true;
	}

	async function handlePower(action: string) {
		await bridge.powerAction(action);
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div
	class="flex w-full flex-col items-center justify-center gap-12"
	class:animate-success-fade={success}
	style:height={cfg?.debug ? `${100 - debugPanelHeight}vh` : '100%'}
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
				<select
					bind:value={selectedSessionIdx}
					class="cursor-pointer appearance-none rounded-md border border-white/10 bg-white/5 px-3 py-1.5 text-xs text-text-muted outline-none transition-colors hover:border-white/20 hover:text-text"
				>
					{#each sessions as session, i}
						<option value={String(i)} class="bg-bg text-text">
							{session.name} ({session.type})
						</option>
					{/each}
				</select>
				{#if selectedSession}
					<span
						class="session-badge rounded-full px-2 py-0.5 text-[10px] font-medium uppercase tracking-wider"
						class:session-badge-wayland={selectedSession.type === 'wayland'}
						class:session-badge-x11={selectedSession.type !== 'wayland'}
					>
						{selectedSession.type}
					</span>
				{/if}
			</div>
		{/if}
	</form>
</div>

{#if powerEnabled}
	<div class="fixed right-6 flex items-center gap-1" style:bottom={cfg?.debug ? `calc(${debugPanelHeight}vh + 1.5rem)` : '1.5rem'}>
		<button
			onclick={() => handlePower('suspend')}
			class="power-btn"
			title="Suspend"
		>
			<Moon size={18} />
		</button>
		<button
			onclick={() => handlePower('reboot')}
			class="power-btn"
			title="Reboot"
		>
			<RotateCcw size={18} />
		</button>
		<button
			onclick={() => handlePower('poweroff')}
			class="power-btn"
			title="Power Off"
		>
			<Power size={18} />
		</button>
	</div>
{/if}

{#if cfg?.debug}
	<div class="debug-panel" style:height={`${debugPanelHeight}vh`}>
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="debug-drag-handle" class:debug-drag-active={dragging} onpointerdown={onDragStart}>
			<div class="debug-drag-grip"></div>
		</div>
		<div class="debug-panel-body">
			<div class="debug-pane">
				<div class="debug-pane-header">Config</div>
				<pre class="debug-pane-content">{JSON.stringify(cfg, null, 2)}</pre>
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
