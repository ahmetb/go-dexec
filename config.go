package dexec

import "time"

type Config struct {
	Namespace       string
	ContainerConfig ContainerConfig
	NetworkConfig   NetworkConfig
	TaskConfig      TaskConfig
	CommandDetails  CommandDetails
}

type Mount struct {
	Type        string
	Source      string
	Destination string
	Options     []string
}

type ContainerConfig struct {
	Image  string
	User   string
	Env    []string
	Mounts []Mount
}

type TaskConfig struct {
	Executable string
	Args       []string
	Timeout    time.Duration
	WorkingDir string
}

type NetworkConfig struct {
	DNS []string
}

type CommandDetails struct {
	ExecutorId      int64
	ChainExecutorId int64
	ResultId        int64
}
