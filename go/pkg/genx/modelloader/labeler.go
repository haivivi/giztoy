package modelloader

import (
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/genx/labelers"
)

func registerLabelerBySchema(cfg ConfigFile) ([]string, error) {
	var names []string
	for _, m := range cfg.Models {
		if m.Name == "" {
			return nil, fmt.Errorf("labeler entry missing name")
		}
		generator := m.Model
		if generator == "" {
			return nil, fmt.Errorf("labeler entry %q missing model (generator pattern)", m.Name)
		}

		labeler := labelers.NewGenX(labelers.Config{Generator: generator})
		if err := labelers.Handle(m.Name, labeler); err != nil {
			return nil, fmt.Errorf("register labeler %q: %w", m.Name, err)
		}
		names = append(names, m.Name)
	}
	return names, nil
}
