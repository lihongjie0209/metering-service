package app

import (
	"github.com/lihongjie0209/metering-service/internal/metering"
	"go.uber.org/fx"
)

var MeteringModule = fx.Module("metering", fx.Provide(metering.NewRepository, metering.NewService))
