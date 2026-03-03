import {
  createGreeterServiceClient,
  AuthOutcome,
  AuthMessageType,
  SessionType,
} from "@greetdeez/proto";

export const client = createGreeterServiceClient({
  dev: { // Mock client impl for local dev
    createSession: async () => ({
      outcome: AuthOutcome.AUTH_MESSAGE,
      authMessage: { type: AuthMessageType.SECRET, message: "Password:" },
    }),
    postAuth: async () => ({
      outcome: AuthOutcome.SUCCESS,
    }),
    listSessions: async () => ({
      sessions: [
        { name: "Hyprland", cmd: ["Hyprland"], type: SessionType.WAYLAND, desktop: "Hyprland" },
        { name: "Sway", cmd: ["sway"], type: SessionType.WAYLAND, desktop: "sway" },
        { name: "i3", cmd: ["i3"], type: SessionType.X11, desktop: "i3" },
      ],
    }),
    getSystemInfo: async () => ({
      info: { hostname: "dev-machine" },
    }),
    getPowerCapabilities: async () => ({
      capabilities: { canPoweroff: true, canReboot: true, canSuspend: true },
    }),
    getState: async () => ({
      state: { lastUser: "demo", lastSession: "Sway" },
    }),
    saveState: async () => ({ ok: true, error: "" }),
    startSession: async () => ({ ok: true, error: "" }),
  },
});
