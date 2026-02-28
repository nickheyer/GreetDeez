export interface Session {
	name: string;
	cmd: string[];
	type: 'wayland' | 'x11';
	desktop: string;
}

export interface Result {
	ok: boolean;
	error?: string;
}

export interface LoginResult {
	ok: boolean;
	error?: string;
	messages?: string[];
}

export interface ThemeConfig {
	accent_color: string;
	aurora_speed: number;
}

export interface PowerConfig {
	enabled: boolean;
}

export interface AppConfig {
	power: PowerConfig;
	theme: ThemeConfig;
}

export interface AppState {
	last_user?: string;
	last_session?: string;
}

const isWebview = typeof globalThis.getSessions === 'function';

const devStubs = {
	getSessions: async (): Promise<Session[]> => [
		{ name: 'Hyprland', cmd: ['Hyprland'], type: 'wayland', desktop: 'Hyprland' },
		{ name: 'Sway', cmd: ['sway'], type: 'wayland', desktop: 'sway' },
		{ name: 'i3', cmd: ['i3'], type: 'x11', desktop: 'i3' }
	],
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	login: async (username: string, _password: string): Promise<LoginResult> => {
		console.log(`[dev] login(${username}, ***)`);
		await new Promise((r) => setTimeout(r, 800));
		if (username === 'fail') return { ok: false, error: 'Invalid credentials' };
		return { ok: true, messages: ['Welcome back!'] };
	},
	startSession: async (sess: Session): Promise<Result> => {
		console.log(`[dev] startSession`, sess);
		await new Promise((r) => setTimeout(r, 500));
		return { ok: true };
	},
	getHostname: async (): Promise<string> => 'dev-machine',
	powerAction: async (action: string): Promise<Result> => {
		console.log(`[dev] powerAction(${action})`);
		return { ok: true };
	},
	getConfig: async (): Promise<AppConfig> => ({
		power: { enabled: true },
		theme: { accent_color: '', aurora_speed: 1.0 }
	}),
	getLastState: async (): Promise<AppState> => ({
		last_user: 'demo',
		last_session: 'Sway'
	}),
	saveState: async (s: AppState): Promise<Result> => {
		console.log(`[dev] saveState`, s);
		return { ok: true };
	}
};

declare global {
	function getSessions(): Promise<Session[]>;
	function login(username: string, password: string): Promise<LoginResult>;
	function startSession(sess: Session): Promise<Result>;
	function getHostname(): Promise<string>;
	function powerAction(action: string): Promise<Result>;
	function getConfig(): Promise<AppConfig>;
	function getLastState(): Promise<AppState>;
	function saveState(s: AppState): Promise<Result>;
}

export const bridge = isWebview
	? {
			getSessions: () => globalThis.getSessions(),
			login: (username: string, password: string) => globalThis.login(username, password),
			startSession: (sess: Session) => globalThis.startSession(sess),
			getHostname: () => globalThis.getHostname(),
			powerAction: (action: string) => globalThis.powerAction(action),
			getConfig: () => globalThis.getConfig(),
			getLastState: () => globalThis.getLastState(),
			saveState: (s: AppState) => globalThis.saveState(s)
		}
	: devStubs;
