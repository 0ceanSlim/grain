package config

type ResourceLimits struct {
	CPUCores   int `yaml:"cpu_cores"`
	MemoryMB   int `yaml:"memory_mb"`
	HeapSizeMB int `yaml:"heap_size_mb"`
}
