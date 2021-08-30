package metrics

import (
	"fmt"
	"time"
)

type HttpServerConfig struct {
	Port           uint
	Interface      string
	RequestTimeout time.Duration
}

func (hs *HttpServerConfig) ListenAddress() string {
	return fmt.Sprintf("%s:%d", hs.Interface, hs.Port)
}
