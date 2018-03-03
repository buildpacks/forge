package engine

import "time"

type ContainerConfig struct {
	Name string

	// Internal
	Hostname   string
	User       string
	Image      string
	WorkingDir string
	Env        []string
	Entrypoint []string
	Cmd        []string

	// External
	Binds        []string
	NetContainer string
	HostIP       string
	HostPort     string
	Memory       int64
	DiskQuota    int64

	// Healthcheck
	Test        []string
	Interval    time.Duration
	Timeout     time.Duration
	StartPeriod time.Duration
	Retries     int

	// Control
	Exit  <-chan struct{}  // default: inherit from engine
	Check <-chan time.Time // default: 1 second intervals
}

type RegistryCreds struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	Email         string `json:"email"`
	ServerAddress string `json:"serveraddress"`
}
