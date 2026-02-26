<script lang="ts">
	import { bridge, type Session, type AppConfig } from '$lib/bridge';
	import { onMount } from 'svelte';
	import { LoaderCircle, Power, RotateCcw, Moon } from '@lucide/svelte';

	let username = $state('');
	let password = $state('');
	let sessions = $state<Session[]>([]);
	let selectedSessionIdx = $state(0);
	let errorMsg = $state('');
	let status = $state<'idle' | 'authenticating' | 'starting'>('idle');
	let hostname = $state('');
	let now = $state(new Date());
	let shaking = $state(false);
	let success = $state(false);
	let cfg = $state<AppConfig | null>(null);

	let passwordInput: HTMLInputElement | undefined = $state();
	let usernameInput: HTMLInputElement | undefined = $state();

	let selectedSession = $derived(sessions[selectedSessionIdx]);
	let time = $derived(now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
	let date = $derived(
		now.toLocaleDateString([], { weekday: 'long', month: 'long', day: 'numeric' })
	);
	let powerEnabled = $derived(cfg?.power?.enabled ?? true);

	onMount(() => {
		bridge.getSessions().then((s) => (sessions = s));
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
		usernameInput?.focus();
		return () => clearInterval(timer);
	});

	function handleUsernameKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && username) {
			e.preventDefault();
			passwordInput?.focus();
		}
	}

	async function handleLogin(e: SubmitEvent) {
		e.preventDefault();
		if (!username || !password || !selectedSession) return;

		errorMsg = '';
		status = 'authenticating';

		const authResult = await bridge.login(username, password);
		if (!authResult.ok) {
			errorMsg = authResult.error ?? 'Authentication failed';
			status = 'idle';
			password = '';
			shaking = true;
			setTimeout(() => (shaking = false), 500);
			setTimeout(() => (errorMsg = ''), 4000);
			passwordInput?.focus();
			return;
		}

		status = 'starting';
		const startResult = await bridge.startSession(selectedSession.cmd);
		if (!startResult.ok) {
			errorMsg = startResult.error ?? 'Failed to start session';
			status = 'idle';
			shaking = true;
			setTimeout(() => (shaking = false), 500);
			return;
		}

		success = true;
	}

	async function handlePower(action: string) {
		await bridge.powerAction(action);
	}
</script>

<div class="flex h-full w-full flex-col items-center justify-center gap-12" class:animate-success-fade={success}>
	<div class="animate-fade-up flex flex-col items-center gap-1 delay-100">
		<span class="clock-glow text-7xl font-extralight tracking-wide">
			{time}
		</span>
		<span class="text-base text-[var(--color-text-muted)]">{date}</span>
	</div>

	<form
		onsubmit={handleLogin}
		class="login-panel animate-fade-up flex w-[380px] flex-col items-center gap-4 rounded-2xl border border-white/[0.08] p-8 backdrop-blur-xl delay-300"
		class:animate-shake={shaking}
	>
		<p class="mb-2 text-xs font-medium tracking-[0.2em] uppercase text-[var(--color-text-muted)]">
			{hostname}
		</p>

		<input
			bind:this={usernameInput}
			type="text"
			placeholder="Username"
			bind:value={username}
			onkeydown={handleUsernameKeydown}
			disabled={status !== 'idle'}
			autocomplete="off"
			class="input-field"
		/>

		<input
			bind:this={passwordInput}
			type="password"
			placeholder="Password"
			bind:value={password}
			disabled={status !== 'idle'}
			class="input-field"
		/>

		{#if errorMsg}
			<p class="animate-slide-in text-sm text-[var(--color-error)]">{errorMsg}</p>
		{/if}

		<button
			type="submit"
			disabled={status !== 'idle'}
			class="flex w-full items-center justify-center gap-2 rounded-lg bg-white/10 py-3 text-sm font-medium text-[var(--color-text)] transition-all duration-200 hover:bg-white/15 disabled:cursor-not-allowed disabled:opacity-40"
		>
			{#if status === 'authenticating'}
				<LoaderCircle size={16} class="animate-spin-slow" />
				Authenticating...
			{:else if status === 'starting'}
				<LoaderCircle size={16} class="animate-spin-slow" />
				Starting session...
			{:else}
				Log in
			{/if}
		</button>

		{#if sessions.length > 0}
			<div class="flex w-full items-center justify-center gap-2 pt-1">
				<select
					bind:value={selectedSessionIdx}
					class="cursor-pointer appearance-none rounded-md border border-white/10 bg-white/[0.05] px-3 py-1.5 text-xs text-[var(--color-text-muted)] outline-none transition-colors hover:border-white/20 hover:text-[var(--color-text)]"
				>
					{#each sessions as session, i}
						<option value={i} class="bg-[var(--color-bg)] text-[var(--color-text)]">
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
	<div class="fixed right-6 bottom-6 flex items-center gap-1">
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
