package modelloader

import (
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/genx/profilers"
)

func registerProfilerBySchema(cfg ConfigFile) ([]string, error) {
	var names []string
	for _, m := range cfg.Models {
		if m.Name == "" {
			return nil, fmt.Errorf("profiler entry missing name")
		}
		generator := m.Model
		if generator == "" {
			return nil, fmt.Errorf("profiler entry %q missing model (generator pattern)", m.Name)
		}

		prof := profilers.NewGenX(profilers.Config{
			Generator: generator,
		})
		if err := profilers.Handle(m.Name, prof); err != nil {
			return nil, fmt.Errorf("register profiler %q: %w", m.Name, err)
		}
		names = append(names, m.Name)
	}
	return names, nil
}
