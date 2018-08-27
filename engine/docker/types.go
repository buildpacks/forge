package docker

type CreateContainerConfig struct {
	Hostname        string   `json:"Hostname"`
	Domainname      string   `json:"Domainname"`
	User            string   `json:"User"`
	AttachStdin     bool     `json:"AttachStdin"`
	AttachStdout    bool     `json:"AttachStdout"`
	AttachStderr    bool     `json:"AttachStderr"`
	Tty             bool     `json:"Tty"`
	OpenStdin       bool     `json:"OpenStdin"`
	StdinOnce       bool     `json:"StdinOnce"`
	Env             []string `json:"Env"`
	Cmd             []string `json:"Cmd"`
	Healthcheck     *HealthConfig
	Entrypoint      []string               `json:"Entrypoint"`
	Image           string                 `json:"Image"`
	Labels          map[string]string      `json:"Labels"`
	Volumes         map[string]interface{} `json:"Volumes"`
	WorkingDir      string                 `json:"WorkingDir"`
	NetworkDisabled bool                   `json:"NetworkDisabled"`
	MacAddress      string                 `json:"MacAddress"`
	ExposedPorts    map[string]interface{} `json:"ExposedPorts"`
	StopSignal      string                 `json:"StopSignal"`
	StopTimeout     int                    `json:"StopTimeout"`
	HostConfig      HostConfig             `json:"HostConfig"`
	// NetworkingConfig NetworkingConfig       `json:"NetworkingConfig"`
}

type HealthConfig struct {
	Test        []string
	Interval    int64
	Timeout     int64
	Retries     int
	StartPeriod int64
}

type HostConfig struct {
	Binds                []string                       `json:"Binds"`
	Links                []string                       `json:"Links"`
	Memory               int                            `json:"Memory"`
	MemorySwap           int                            `json:"MemorySwap"`
	MemoryReservation    int                            `json:"MemoryReservation"`
	KernelMemory         int                            `json:"KernelMemory"`
	NanoCPUs             int                            `json:"NanoCPUs"`
	CPUPercent           int                            `json:"CpuPercent"`
	CPUShares            int                            `json:"CpuShares"`
	CPUPeriod            int                            `json:"CpuPeriod"`
	CPURealtimePeriod    int                            `json:"CpuRealtimePeriod"`
	CPURealtimeRuntime   int                            `json:"CpuRealtimeRuntime"`
	CPUQuota             int                            `json:"CpuQuota"`
	CpusetCpus           string                         `json:"CpusetCpus"`
	CpusetMems           string                         `json:"CpusetMems"`
	MaximumIOps          int                            `json:"MaximumIOps"`
	MaximumIOBps         int                            `json:"MaximumIOBps"`
	BlkioWeight          int                            `json:"BlkioWeight"`
	BlkioWeightDevice    interface{}                    `json:"BlkioWeightDevice"`
	BlkioDeviceReadBps   interface{}                    `json:"BlkioDeviceReadBps"`
	BlkioDeviceReadIOps  interface{}                    `json:"BlkioDeviceReadIOps"`
	BlkioDeviceWriteBps  interface{}                    `json:"BlkioDeviceWriteBps"`
	BlkioDeviceWriteIOps interface{}                    `json:"BlkioDeviceWriteIOps"`
	MemorySwappiness     int                            `json:"MemorySwappiness"`
	OomKillDisable       bool                           `json:"OomKillDisable"`
	OomScoreAdj          int                            `json:"OomScoreAdj"`
	PidMode              string                         `json:"PidMode"`
	PidsLimit            int                            `json:"PidsLimit"`
	PortBindings         map[string][]PortBindingConfig `json:"PortBindings"`
	PublishAllPorts      bool                           `json:"PublishAllPorts"`
	Privileged           bool                           `json:"Privileged"`
	ReadonlyRootfs       bool                           `json:"ReadonlyRootfs"`
	DNS                  []string                       `json:"Dns"`
	DNSOptions           []string                       `json:"DnsOptions"`
	DNSSearch            []string                       `json:"DnsSearch"`
	VolumesFrom          []string                       `json:"VolumesFrom"`
	CapAdd               []string                       `json:"CapAdd"`
	CapDrop              []string                       `json:"CapDrop"`
	GroupAdd             []string                       `json:"GroupAdd"`
	RestartPolicy        struct {
		Name              string `json:"Name"`
		MaximumRetryCount int    `json:"MaximumRetryCount"`
	} `json:"RestartPolicy"`
	AutoRemove  bool          `json:"AutoRemove"`
	NetworkMode string        `json:"NetworkMode"`
	Devices     []interface{} `json:"Devices"`
	Ulimits     []struct {
	} `json:"Ulimits"`
	LogConfig struct {
		Type   string `json:"Type"`
		Config struct {
		} `json:"Config"`
	} `json:"LogConfig"`
	SecurityOpt []interface{} `json:"SecurityOpt"`
	StorageOpt  struct {
	} `json:"StorageOpt"`
	CgroupParent string `json:"CgroupParent"`
	VolumeDriver string `json:"VolumeDriver"`
	ShmSize      int    `json:"ShmSize"`
}

// type NetworkingConfig struct {
// 	EndpointsConfig struct {
// 		IsolatedNW struct {
// 			IPAMConfig struct {
// 				IPv4Address  string   `json:"IPv4Address"`
// 				IPv6Address  string   `json:"IPv6Address"`
// 				LinkLocalIPs []string `json:"LinkLocalIPs"`
// 			} `json:"IPAMConfig"`
// 			Links   []string `json:"Links"`
// 			Aliases []string `json:"Aliases"`
// 		} `json:"isolated_nw"`
// 	} `json:"EndpointsConfig"`
// }

type PortBindingConfig struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}
