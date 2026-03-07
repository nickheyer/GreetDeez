<script lang="ts">
	import { client } from '$lib/client';
	import { SessionType, PowerAction, type Session, type PowerCapabilities } from '@nickheyer/greetdeez';
	import { onMount } from 'svelte';
	import { LoaderCircle, Power, RotateCcw, Moon, TriangleAlert, ChevronDown } from '@lucide/svelte';

	let username = $state('');
	let password = $state('');
	let sessions = $state<Session[]>([]);
	let selectedIdx = $state(0);
	let errorMsg = $state('');
	let status = $state<'idle' | 'authenticating' | 'starting' | 'cooldown'>('idle');
	let hostname = $state('');
	let now = $state(new Date());
	let shaking = $state(false);
	let success = $state(false);
	let capsLock = $state(false);
	let powerCaps = $state<PowerCapabilities | null>(null);
	let dropOpen = $state(false);

	let passwordInput: HTMLInputElement | undefined = $state();
	let usernameInput: HTMLInputElement | undefined = $state();

	let selectedSession = $derived(sessions[selectedIdx]);
	let time = $derived(now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
	let date = $derived(
		now.toLocaleDateString([], { weekday: 'long', month: 'long', day: 'numeric' })
	);
	let powerEnabled = $derived(
		powerCaps != null && (powerCaps.canPoweroff || powerCaps.canReboot || powerCaps.canSuspend)
	);
	let busy = $derived(status !== 'idle');

	function sessionLabel(s: Session): string {
		return SessionType[s.type];
	}

	function fail(msg: string, cooldown = false) {
		errorMsg = msg;
		shaking = true;
		setTimeout(() => (shaking = false), 500);
		if (cooldown) {
			status = 'cooldown';
			password = '';
			setTimeout(() => (errorMsg = ''), 4000);
			setTimeout(() => {
				status = 'idle';
				passwordInput?.focus();
			}, 2000);
		} else {
			status = 'idle';
		}
	}

	onMount(() => {
		async function init() {
			const [sessResp, stateResp, sysResp, powerResp] = await Promise.all([
				client.listSessions(),
				client.getState(),
				client.getSystemInfo(),
				client.getPowerCapabilities()
			]);

			sessions = sessResp.sessions;
			hostname = sysResp.info.hostname;
			powerCaps = powerResp.capabilities;

			const st = stateResp.state;
			if (st.lastUser) username = st.lastUser;
			if (st.lastSession && sessions.length > 0) {
				const idx = sessions.findIndex((s) => s.name === st.lastSession);
				if (idx >= 0) selectedIdx = idx;
			}

			(username ? passwordInput : usernameInput)?.focus();
		}
		init();

		const timer = setInterval(() => (now = new Date()), 1000);
		return () => clearInterval(timer);
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
		status = 'authenticating';

		try {
			const auth = await client.authenticate({ username, password });
			if (!auth.success) return fail(auth.error || 'Authentication failed', true);

			status = 'starting';
			const start = await client.startSession({ session: selectedSession });
			if (!start.success) return fail(start.error || 'Failed to start session');

			client.saveState({ state: { lastUser: username, lastSession: selectedSession.name } });
			success = true;
		} catch (err) {
			fail(err instanceof Error ? err.message : 'Login failed', true);
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div
	class="flex h-full w-full flex-col items-center justify-center gap-12"
	class:animate-success-fade={success}
>
	<div class="animate-fade-up flex flex-col items-center gap-1 delay-100">
		<span class="clock-glow text-7xl font-extralight tracking-wide">{time}</span>
		<span class="text-base text-text-muted">{date}</span>
	</div>

	<form
		onsubmit={handleLogin}
		class="login-panel animate-fade-up flex w-95 flex-col items-center gap-4 rounded-2xl border border-white/8 p-8 delay-300"
		class:animate-shake={shaking}
	>
		<p class="mb-2 text-xs font-medium tracking-[0.2em] uppercase text-text-muted">{hostname}</p>

		<input
			bind:this={usernameInput}
			type="text"
			placeholder="Username"
			bind:value={username}
			onkeydown={handleUsernameKeydown}
			disabled={busy}
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
				disabled={busy}
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

		<button
			type="submit"
			disabled={busy}
			class="flex w-full items-center justify-center gap-2 rounded-lg bg-white/10 py-3 text-sm font-medium text-text transition-all duration-200 hover:bg-white/15 disabled:cursor-not-allowed disabled:opacity-40"
		>
			{#if status === 'authenticating'}
				<LoaderCircle size={16} class="animate-spin-slow" /> Authenticating...
			{:else if status === 'starting'}
				<LoaderCircle size={16} class="animate-spin-slow" /> Starting session...
			{:else if status === 'cooldown'}
				<LoaderCircle size={16} class="animate-spin-slow" /> Please wait...
			{:else}
				Log in
			{/if}
		</button>

		{#if sessions.length > 1}
			<div class="session-dropdown">
				<button
					type="button"
					class="session-dropdown-trigger"
					onclick={() => (dropOpen = !dropOpen)}
				>
					{sessionLabel(selectedSession)}
					<ChevronDown size={12} class="session-dropdown-chevron" />
				</button>
				{#if dropOpen}
					<div class="session-dropdown-backdrop" onclick={() => (dropOpen = false)}></div>
					<div class="session-dropdown-menu">
						{#each sessions as s, i}
							<button
								type="button"
								class="session-dropdown-item"
								class:session-dropdown-item-active={i === selectedIdx}
								onclick={() => {
									selectedIdx = i;
									dropOpen = false;
								}}>{sessionLabel(s)}</button
							>
						{/each}
					</div>
				{/if}
			</div>
		{:else if sessions.length === 1}
			<span class="session-badge session-badge-name">{sessionLabel(selectedSession)}</span>
		{/if}
	</form>
</div>

{#if powerEnabled}
	<div class="fixed right-6 bottom-6 flex items-center gap-1">
		{#if powerCaps?.canSuspend}
			<button
				onclick={() => client.executePowerAction({ action: PowerAction.SUSPEND })}
				class="power-btn"
				title="Suspend"
			>
				<Moon size={18} />
			</button>
		{/if}
		{#if powerCaps?.canReboot}
			<button
				onclick={() => client.executePowerAction({ action: PowerAction.REBOOT })}
				class="power-btn"
				title="Reboot"
			>
				<RotateCcw size={18} />
			</button>
		{/if}
		{#if powerCaps?.canPoweroff}
			<button
				onclick={() => client.executePowerAction({ action: PowerAction.POWEROFF })}
				class="power-btn"
				title="Power Off"
			>
				<Power size={18} />
			</button>
		{/if}
	</div>
{/if}
