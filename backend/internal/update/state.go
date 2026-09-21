package update

import "fmt"

// Update lifecycle states. The backend owns all transitions; the UI
// renders them verbatim and keeps no side state machine.
const (
	StateIdle            = "idle"
	StateChecking        = "checking"
	StateNoUpdate        = "no-update"
	StateUpdateAvailable = "update-available"
	StateDownloading     = "downloading"
	StateVerifying       = "verifying"
	StateStaging         = "staging"
	StateInstalling      = "installing"
	StateRestarting      = "restarting"
	StateUpdated         = "updated"
	StateCancelled       = "cancelled"
	StateFailed          = "failed"
	StateRolledBack      = "rolled-back"
)

// allowed transitions; anything else is rejected loudly.
var allowed = map[string]map[string]bool{
	StateIdle: {
		StateChecking: true,
	},
	StateChecking: {
		StateNoUpdate: true, StateUpdateAvailable: true,
		StateFailed: true, StateCancelled: true,
	},
	StateNoUpdate: {
		StateChecking: true,
	},
	StateUpdateAvailable: {
		// Downloading a fresh artifact, or Staging (assemble) an
		// already-verified download when the user presses Install.
		StateDownloading: true, StateStaging: true,
		StateChecking:  true,
		StateCancelled: true, StateIdle: true, // dismissed
	},
	StateDownloading: {
		StateVerifying: true, StateFailed: true, StateCancelled: true,
	},
	StateVerifying: {
		// Straight to Staging for auto flows; back to UpdateAvailable
		// (verified, awaiting the user's Install) for manual flows.
		StateStaging: true, StateUpdateAvailable: true,
		StateFailed: true, StateCancelled: true,
	},
	StateStaging: {
		StateInstalling: true, StateFailed: true, StateCancelled: true,
	},
	StateInstalling: {
		StateRestarting: true, StateFailed: true, StateRolledBack: true,
	},
	StateRestarting: {
		StateUpdated: true, StateFailed: true, StateRolledBack: true,
	},
	StateUpdated: {
		StateChecking: true,
	},
	StateCancelled: {
		StateChecking: true, StateIdle: true,
	},
	StateFailed: {
		StateChecking: true, StateIdle: true,
	},
	StateRolledBack: {
		StateChecking: true, StateIdle: true,
	},
}

// Transition validates from→to.
func Transition(from, to string) error {
	if allowed[from][to] {
		return nil
	}
	return fmt.Errorf("update: impossible transition %q → %q", from, to)
}

// Progress is the live download/install picture. All backend-measured;
// the UI never invents numbers.
type Progress struct {
	BytesDone  int64   `json:"bytesDone"`
	BytesTotal int64   `json:"bytesTotal"`
	Percent    float64 `json:"percent"`
	SpeedBps   int64   `json:"speedBps,omitempty"`
	ETASeconds int64   `json:"etaSeconds,omitempty"`
}

// Info is the wire view of an available update for the UI.
type Info struct {
	Version      string   `json:"version"`
	Channel      string   `json:"channel"`
	ReleaseDate  string   `json:"releaseDate,omitempty"`
	ReleaseNotes []string `json:"releaseNotes,omitempty"`
	SizeBytes    int64    `json:"sizeBytes"`
	ArtifactType string   `json:"artifactType"`
	MinVersion   string   `json:"minVersion,omitempty"`
}

// Status is the full snapshot the UI renders and tests assert on.
type Status struct {
	State         string   `json:"state"`
	Info          *Info    `json:"info,omitempty"`
	Progress      Progress `json:"progress"`
	Error         string   `json:"error,omitempty"`
	Channel       string   `json:"channel"`
	Current       string   `json:"currentVersion"`
	LastCheckUnix int64    `json:"lastCheckUnix,omitempty"`
	// Staged is true once the download is verified and assembled-ready:
	// the UI shows Install instead of Download.
	Staged bool `json:"staged"`
}
