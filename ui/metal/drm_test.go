package metal

import (
	"testing"
	"unsafe"
)

// kernel abi is exact soooo drift here corrupts ioctls
func TestDRMStructSizes(t *testing.T) {
	cases := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"drm_mode_modeinfo", unsafe.Sizeof(drmModeInfo{}), 68},
		{"drm_mode_card_res", unsafe.Sizeof(drmCardRes{}), 64},
		{"drm_mode_get_connector", unsafe.Sizeof(drmGetConnector{}), 80},
		{"drm_mode_get_encoder", unsafe.Sizeof(drmGetEncoder{}), 20},
		{"drm_mode_crtc", unsafe.Sizeof(drmModeCrtc{}), 104},
		{"drm_mode_create_dumb", unsafe.Sizeof(drmCreateDumb{}), 32},
		{"drm_mode_map_dumb", unsafe.Sizeof(drmMapDumb{}), 16},
		{"drm_mode_fb_cmd", unsafe.Sizeof(drmFbCmd{}), 28},
		{"drm_mode_crtc_page_flip", unsafe.Sizeof(drmPageFlip{}), 24},
		{"drm_mode_destroy_dumb", unsafe.Sizeof(drmDestroyDumb{}), 4},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s size = %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestConnectorName(t *testing.T) {
	// names must match the kernel's so users can copy them from wlr-randr etc
	cases := []struct {
		typ, id uint32
		want    string
	}{
		{10, 1, "DP-1"},
		{11, 2, "HDMI-A-2"},
		{14, 1, "eDP-1"},
		{15, 1, "Virtual-1"},
		{99, 1, "Unknown99-1"},
	}
	for _, c := range cases {
		if got := connectorName(c.typ, c.id); got != c.want {
			t.Errorf("connectorName(%d, %d) = %q, want %q", c.typ, c.id, got, c.want)
		}
	}
}

func TestIoctlNumbers(t *testing.T) {
	// spot check against drm.h constants
	if got := drmIOWR(0xA0, 64); got != 0xC040_64A0 {
		t.Errorf("DRM_IOCTL_MODE_GETRESOURCES = %#x, want 0xc04064a0", got)
	}
	if got := drmIOWR(0xB2, 32); got != 0xC020_64B2 {
		t.Errorf("DRM_IOCTL_MODE_CREATE_DUMB = %#x, want 0xc02064b2", got)
	}
	if got := drmIO(0x1e); got != 0x641E {
		t.Errorf("DRM_IOCTL_SET_MASTER = %#x, want 0x641e", got)
	}
}
