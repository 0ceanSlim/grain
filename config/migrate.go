package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// CheckAndMigrateConfig reads the raw YAML file and checks for outdated config
// formats (e.g., the old mongodb section from before the nostrdb migration).
// It prints warnings to stderr since this runs before loggers are initialized.
func CheckAndMigrateConfig(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading config file for migration check: %w", err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parsing config file for migration check: %w", err)
	}

	_, hasMongoDb := raw["mongodb"]
	_, hasDatabase := raw["database"]

	if hasMongoDb {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "WARNING: Your config.yml contains a 'mongodb' section which is no longer used.")
		fmt.Fprintln(os.Stderr, "  Grain has migrated from MongoDB to nostrdb.")
		fmt.Fprintln(os.Stderr, "  Please remove the 'mongodb' section and add a 'database' section:")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "    database:")
		fmt.Fprintln(os.Stderr, "      path: \"data\"")
		fmt.Fprintln(os.Stderr, "      map_size_mb: 4096")
		fmt.Fprintln(os.Stderr, "")

		if !hasDatabase {
			fmt.Fprintln(os.Stderr, "  No 'database' section found — defaults will be applied automatically,")
			fmt.Fprintln(os.Stderr, "  but you should update your config file to avoid this warning.")
			fmt.Fprintln(os.Stderr, "")
		}
	}

	return nil
}
