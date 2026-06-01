package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type rootOptions struct {
	envFile string
}

// NewRootCommand creates the Prompt Gate backend command tree.
func NewRootCommand() *cobra.Command {
	options := rootOptions{}

	root := &cobra.Command{
		Use:           "promptgate",
		Short:         "Prompt Gate backend service",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			workingDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve working directory: %w", err)
			}

			return loadEnvFile(workingDir, options.envFile)
		},
	}

	root.PersistentFlags().StringVar(&options.envFile, "env-file", "", "Path to a dotenv file to load before running a command")

	root.AddCommand(
		newAPICommand(),
		newProxyCommand(),
		newMigrateCommand(),
		newScheduleCommand(),
	)

	return root
}

// loadEnvFile loads dotenv values into the process environment without overriding existing values.
func loadEnvFile(startDir string, explicitPath string) error {
	envFilePath, err := resolveEnvFilePath(startDir, explicitPath)
	if err != nil {
		return err
	}
	if envFilePath == "" {
		return nil
	}

	envLoader := viper.New()
	envLoader.SetConfigFile(envFilePath)
	envLoader.SetConfigType("env")

	if err := envLoader.ReadInConfig(); err != nil {
		return fmt.Errorf("load %s: %w", envFilePath, err)
	}

	for _, key := range envLoader.AllKeys() {
		envKey := strings.ToUpper(key)
		if currentValue, exists := os.LookupEnv(envKey); exists && currentValue != "" {
			continue
		}

		if err := os.Setenv(envKey, envLoader.GetString(key)); err != nil {
			return fmt.Errorf("set %s from %s: %w", envKey, envFilePath, err)
		}
	}

	return nil
}

// resolveEnvFilePath returns the explicit dotenv path or searches parent directories for .env.
func resolveEnvFilePath(startDir string, explicitPath string) (string, error) {
	if explicitPath != "" {
		return explicitPath, nil
	}

	currentDir := filepath.Clean(startDir)
	for {
		candidate := filepath.Join(currentDir, ".env")
		info, err := os.Stat(candidate)
		switch {
		case err == nil && !info.IsDir():
			return candidate, nil
		case err == nil && info.IsDir():
			return "", fmt.Errorf("%s is a directory, expected a file", candidate)
		case err != nil && !errors.Is(err, os.ErrNotExist):
			return "", fmt.Errorf("stat %s: %w", candidate, err)
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return "", nil
		}
		currentDir = parentDir
	}
}
