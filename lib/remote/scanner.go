package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"

	"github.com/sirupsen/logrus"
	"github.com/sundowndev/phoneinfoga/v2/lib/number"
)

type ScannerOptions map[string]interface{}

func (o ScannerOptions) GetStringEnv(k string) string {
	if v, ok := o[k].(string); ok {
		return v
	}
	return os.Getenv(k)
}

type Plugin interface {
	Lookup(string) (plugin.Symbol, error)
}

type Scanner interface {
	Name() string
	Description() string
	DryRun(number.Number, ScannerOptions) error
	Run(number.Number, ScannerOptions) (interface{}, error)
}

func OpenPlugin(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("given path %s does not exist", path)
	}

	_, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("given plugin %s is not valid: %v", path, err)
	}

	return nil
}

func LoadPluginDir(dir string) []error {
	info, err := os.Stat(dir)
	if err != nil {
		return []error{fmt.Errorf("plugin directory %s does not exist", dir)}
	}
	if !info.IsDir() {
		return []error{fmt.Errorf("path %s is not a directory", dir)}
	}

	var errs []error
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []error{fmt.Errorf("failed to read plugin directory %s: %v", dir, err)}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".so" {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())
		if err := OpenPlugin(fullPath); err != nil {
			logrus.WithField("plugin", fullPath).WithError(err).Warn("Failed to load plugin, skipping")
			errs = append(errs, fmt.Errorf("failed to load plugin %s: %v", fullPath, err))
			continue
		}
		logrus.WithField("plugin", fullPath).Info("Successfully loaded plugin")
	}

	return errs
}
