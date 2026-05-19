package config

type KindSizeLimitConfig struct {
	Kind    int `yaml:"kind" json:"kind"`
	MaxSize int `yaml:"max_size" json:"max_size"`
}
