// internal/i18n/loader.go — Loads YAML locale files from a directory.
package i18n

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Translations maps language code → key → translated string.
type Translations map[string]map[string]string

// LoadLocales reads all *.yaml files in dir and returns a Translations map.
// Each file is keyed by its stem name (e.g. "en", "ar").
func LoadLocales(dir string) (Translations, error) {
	t := make(Translations)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("i18n: reading locales dir %q: %w", dir, err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}

		lang := strings.TrimSuffix(e.Name(), ".yaml")
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("i18n: reading %s: %w", e.Name(), err)
		}

		var kv map[string]string
		if err := yaml.Unmarshal(data, &kv); err != nil {
			return nil, fmt.Errorf("i18n: parsing %s: %w", e.Name(), err)
		}
		t[lang] = kv
	}

	return t, nil
}

// T looks up a key for the given language, falling back to English, then the key itself.
func (t Translations) T(lang, key string) string {
	if m, ok := t[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	if m, ok := t["en"]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return key
}
