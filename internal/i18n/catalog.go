package i18n

import (
	"embed"
	"fmt"

	"github.com/BurntSushi/toml"
)

//go:embed locales/*.toml
var localeFiles embed.FS

type Catalog struct{ messages map[string]string }

func Load(language string) (*Catalog, error) {
	name := "locales/active." + language + ".toml"
	data, err := localeFiles.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("idioma_nao_suportado")
	}
	var raw struct {
		Messages map[string]string `toml:"messages"`
	}
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return nil, err
	}
	return &Catalog{messages: raw.Messages}, nil
}

func (c *Catalog) T(key string, args ...any) string {
	message, ok := c.messages[key]
	if !ok {
		return "[" + key + "]"
	}
	if len(args) == 0 {
		return message
	}
	return fmt.Sprintf(message, args...)
}
