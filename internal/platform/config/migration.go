package config

// LoadMigration reads database migration configuration from a strict YAML file.
func LoadMigration(configPath string) (MigrationConfig, error) {
	file, err := loadFile(configPath)
	if err != nil {
		return MigrationConfig{}, err
	}
	cfg := MigrationConfig{
		LogConfig:         file.logConfig(),
		DatabaseURLConfig: file.databaseConfig(),
	}
	if err := validateDatabaseConfig(cfg.DatabaseURLConfig); err != nil {
		return MigrationConfig{}, err
	}
	return cfg, nil
}
