package config

type KindSizeLimitConfig struct {
	Kind    int `yaml:"kind"`
	MaxSize int `yaml:"max_size"`
}
