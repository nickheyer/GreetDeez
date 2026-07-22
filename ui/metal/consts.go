package metal

// CHANGE TEXT HERE
const (
	// backdrop scroller
	txtMarquee = "BAAAREEEEMETAAAAALLL  ...  I USE GREETDEEZ BTW <3  ... UNIX SOCKET HAD BABY WITH FRAMEBUFFER  ...  GREETINGS TO THE GREETD CREW  ...  "

	// panel
	txtSubtitle      = "GREETDEEZ // BARE METAL"
	txtLabelLogin    = "LOGIN"
	txtLabelPassword = "PASSWORD"
	txtLabelSession  = "SESSION"

	// clock formats
	fmtClockTime = "15:04"
	fmtClockDate = "Mon 02 Jan"

	// status line
	txtAuthenticating = "AUTHENTICATING"
	txtLaunching      = "LAUNCHING " // + session name
	txtAccessDenied   = "ACCESS DENIED"
	txtAuthFailed     = "authentication failed"
	txtConfirmPower   = "PRESS AGAIN TO CONFIRM  " // + power key name

	// power actions
	txtPoweringOff = "POWERING OFF"
	txtRebooting   = "REBOOTING"
	txtSuspending  = "SUSPENDING"
	txtKeyPoweroff = "F10 = POWER OFF"
	txtKeyReboot   = "F11 = REBOOT"
	txtKeySuspend  = "F12 = SUSPEND"

	// under the panel
	txtHintLogin    = "ENTER LOGIN"
	txtHintSession  = "TAB SESSION"
	txtHintPoweroff = "F10 OFF"
	txtHintReboot   = "F11 REBOOT"
	txtHintSuspend  = "F12 SLEEP"

	// session badges
	txtBadgeWayland = "WAYLAND"
	txtBadgeX11     = "X11"
	txtBadgeTTY     = "TTY"

	// fallbacks when the backend has nothing for us
	txtFallbackHostname = "linux"
	txtFallbackSession  = "shell"
)
