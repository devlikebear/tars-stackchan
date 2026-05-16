package controlui

import (
	"net/http"

	"github.com/devlikebear/tars-stackchan/mcp-server/internal/perception"
)

// perceiveStatus is operational visibility for the perception loop. It is
// read-only and never exposes secrets (no tokens / no raw media).
type perceiveStatus struct {
	BridgeMode     string `json:"bridge_mode"`
	CameraEnabled  bool   `json:"camera_enabled"`
	OwnerEnrolled  bool   `json:"owner_enrolled"`
	OwnerName      string `json:"owner_name,omitempty"`
	OwnerFaces     int    `json:"owner_faces"`
	OwnerVoices    int    `json:"owner_voices"`
	TARSConfigured bool   `json:"tars_configured"`
	TARSChannel    string `json:"tars_channel,omitempty"`
	// CameraNote surfaces the known Spike S limitation so operators are not
	// surprised that vision is unavailable on real CoreS3.
	CameraNote string `json:"camera_note,omitempty"`
}

func (s *Server) handlePerceiveStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	cfg := perception.LoadConfig()
	profile, err := perception.OwnerStore{Dir: cfg.OwnerDir}.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load owner profile: "+err.Error())
		return
	}

	out := perceiveStatus{
		BridgeMode:     s.config.BridgeMode,
		CameraEnabled:  cfg.CameraEnabled,
		OwnerEnrolled:  profile.Enrolled(),
		OwnerName:      profile.Name,
		OwnerFaces:     len(profile.FaceHashes),
		OwnerVoices:    len(profile.VoiceHashes),
		TARSConfigured: cfg.TARSConfigured(),
		TARSChannel:    cfg.TARSWebhookChannel,
	}
	if cfg.CameraEnabled {
		out.CameraNote = "camera capture on real M5Stack CoreS3 currently resets the device (Spike S: shared I2C bus); set TARS_STACKCHAN_PERCEIVE_CAMERA=off for audio-only operation"
	}
	writeJSON(w, http.StatusOK, out)
}
