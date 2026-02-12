package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/haivivi/giztoy/go/pkg/chatgear"
	"github.com/haivivi/giztoy/go/pkg/jsontime"
)

func main() {
	evt := chatgear.NewStateEvent(chatgear.StateReady, time.Now())
	b, _ := json.Marshal(evt)
	fmt.Println("StateEvent:", string(b))

	stats := &chatgear.StatsEvent{
		Time: jsontime.NowEpochMilli(),
	}
	stats.Battery = &chatgear.Battery{Percentage: 100}
	stats.Volume = &chatgear.Volume{Percentage: 50, UpdateAt: jsontime.NowEpochMilli()}
	stats.SystemVersion = &chatgear.SystemVersion{CurrentVersion: "zig-e2e-0.1.0"}
	b2, _ := json.Marshal(stats)
	fmt.Println("StatsEvent:", string(b2))
}
