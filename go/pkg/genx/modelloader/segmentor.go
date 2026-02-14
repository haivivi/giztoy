package modelloader

import (
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/genx/segmentors"
)

func registerSegmentorBySchema(cfg ConfigFile) ([]string, error) {
	var names []string
	for _, m := range cfg.Models {
		if m.Name == "" {
			return nil, fmt.Errorf("segmentor entry missing name")
		}
		generator := m.Model
		if generator == "" {
			return nil, fmt.Errorf("segmentor entry %q missing model (generator pattern)", m.Name)
		}

		seg := segmentors.NewGenX(segmentors.Config{
			Generator: generator,
		})
		if err := segmentors.Handle(m.Name, seg); err != nil {
			return nil, fmt.Errorf("register segmentor %q: %w", m.Name, err)
		}
		names = append(names, m.Name)
	}
	return names, nil
}
