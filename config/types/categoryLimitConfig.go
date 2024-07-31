package config

type CategoryLimitConfig struct {
	Regular                  LimitBurst `yaml:"regular"`
	Replaceable              LimitBurst `yaml:"replaceable"`
	ParameterizedReplaceable LimitBurst `yaml:"parameterized_replaceable"`
	Ephemeral                LimitBurst `yaml:"ephemeral"`
}