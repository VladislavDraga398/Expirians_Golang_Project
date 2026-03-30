package pricing

import (
	"context"
	"math"
	"strconv"
	"strings"

	"google.golang.org/grpc/metadata"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

const (
	metadataWeatherSeverity = "x-weather-severity"
	metadataTrafficSeverity = "x-traffic-severity"
	metadataCourierLoad     = "x-courier-load"
	metadataVehicleType     = "x-delivery-vehicle-type"
)

// Config описывает параметры динамического ценообразования доставки.
type Config struct {
	Enabled bool

	// BaseFeeMinor — базовый delivery fee в минимальных денежных единицах.
	BaseFeeMinor int64

	// Default* — baseline сигналы, применяются при отсутствии metadata override.
	DefaultWeatherSeverity float64
	DefaultTrafficSeverity float64
	DefaultCourierLoad     float64

	// MaxBps коэффициенты в bps (100 bps = 1%).
	WeatherMaxBps int
	TrafficMaxBps int
	LoadMaxBps    int
}

// DefaultConfig возвращает безопасный baseline (фича выключена по умолчанию).
func DefaultConfig() Config {
	return Config{
		Enabled:                false,
		BaseFeeMinor:           0,
		DefaultWeatherSeverity: 0,
		DefaultTrafficSeverity: 0,
		DefaultCourierLoad:     0,
		WeatherMaxBps:          2500, // до +25%
		TrafficMaxBps:          3000, // до +30%
		LoadMaxBps:             2000, // до +20%
	}
}

// Result содержит применённые коэффициенты и итоговый delivery fee.
type Result struct {
	Applied bool

	BaseFeeMinor     int64
	DeliveryFeeMinor int64

	VehicleType domain.VehicleType

	WeatherSeverity float64
	TrafficSeverity float64
	CourierLoad     float64

	WeatherBps int
	TrafficBps int
	LoadBps    int
	TotalBps   int
}

// Calculator рассчитывает динамический delivery fee.
type Calculator struct {
	cfg Config
}

// NewCalculator создаёт калькулятор с нормализованной конфигурацией.
func NewCalculator(cfg Config) *Calculator {
	cfg = normalizeConfig(cfg)
	return &Calculator{cfg: cfg}
}

// Calculate рассчитывает delivery fee на основе baseline-конфига и metadata.
func (c *Calculator) Calculate(ctx context.Context) Result {
	if c == nil || !c.cfg.Enabled || c.cfg.BaseFeeMinor <= 0 {
		return Result{}
	}

	vehicleType, weatherSeverity, trafficSeverity, courierLoad := c.signalsFromContext(ctx)
	weatherSeverity = clampUnit(weatherSeverity)
	trafficSeverity = clampUnit(trafficSeverity)
	courierLoad = clampUnit(courierLoad)

	weatherBps, trafficBps := c.vehicleFactorBps(vehicleType, weatherSeverity, trafficSeverity)
	loadBps := int(math.Round(courierLoad * float64(c.cfg.LoadMaxBps)))

	totalBps := 10000 + weatherBps + trafficBps + loadBps
	if totalBps < 0 {
		totalBps = 0
	}

	feeMinor := int64(math.Round(float64(c.cfg.BaseFeeMinor) * float64(totalBps) / 10000.0))
	if feeMinor < 0 {
		feeMinor = 0
	}

	return Result{
		Applied:          feeMinor > 0,
		BaseFeeMinor:     c.cfg.BaseFeeMinor,
		DeliveryFeeMinor: feeMinor,
		VehicleType:      vehicleType,
		WeatherSeverity:  weatherSeverity,
		TrafficSeverity:  trafficSeverity,
		CourierLoad:      courierLoad,
		WeatherBps:       weatherBps,
		TrafficBps:       trafficBps,
		LoadBps:          loadBps,
		TotalBps:         totalBps,
	}
}

func (c *Calculator) signalsFromContext(ctx context.Context) (domain.VehicleType, float64, float64, float64) {
	vehicleType := domain.VehicleType("")
	weatherSeverity := c.cfg.DefaultWeatherSeverity
	trafficSeverity := c.cfg.DefaultTrafficSeverity
	courierLoad := c.cfg.DefaultCourierLoad

	readSignals := func(md metadata.MD) {
		if md == nil {
			return
		}

		if value, ok := firstMetadataFloat(md, metadataWeatherSeverity); ok {
			weatherSeverity = value
		}
		if value, ok := firstMetadataFloat(md, metadataTrafficSeverity); ok {
			trafficSeverity = value
		}
		if value, ok := firstMetadataFloat(md, metadataCourierLoad); ok {
			courierLoad = value
		}
		if value, ok := firstMetadataString(md, metadataVehicleType); ok {
			vehicleType = parseVehicleType(value)
		}
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		readSignals(md)
	}
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		readSignals(md)
	}

	return vehicleType, weatherSeverity, trafficSeverity, courierLoad
}

func (c *Calculator) vehicleFactorBps(vehicleType domain.VehicleType, weatherSeverity, trafficSeverity float64) (int, int) {
	switch vehicleType {
	case domain.VehicleTypeScooter, domain.VehicleTypeBike:
		return int(math.Round(weatherSeverity * float64(c.cfg.WeatherMaxBps))), 0
	case domain.VehicleTypeCar:
		return 0, int(math.Round(trafficSeverity * float64(c.cfg.TrafficMaxBps)))
	default:
		return int(math.Round(weatherSeverity * float64(c.cfg.WeatherMaxBps))),
			int(math.Round(trafficSeverity * float64(c.cfg.TrafficMaxBps)))
	}
}

func normalizeConfig(cfg Config) Config {
	defaults := DefaultConfig()

	if cfg.WeatherMaxBps < 0 {
		cfg.WeatherMaxBps = defaults.WeatherMaxBps
	}
	if cfg.TrafficMaxBps < 0 {
		cfg.TrafficMaxBps = defaults.TrafficMaxBps
	}
	if cfg.LoadMaxBps < 0 {
		cfg.LoadMaxBps = defaults.LoadMaxBps
	}
	if cfg.BaseFeeMinor < 0 {
		cfg.BaseFeeMinor = 0
	}

	cfg.DefaultWeatherSeverity = clampUnit(cfg.DefaultWeatherSeverity)
	cfg.DefaultTrafficSeverity = clampUnit(cfg.DefaultTrafficSeverity)
	cfg.DefaultCourierLoad = clampUnit(cfg.DefaultCourierLoad)
	return cfg
}

func clampUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func firstMetadataFloat(md metadata.MD, key string) (float64, bool) {
	value, ok := firstMetadataString(md, key)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func firstMetadataString(md metadata.MD, key string) (string, bool) {
	values := md.Get(key)
	if len(values) == 0 {
		return "", false
	}
	value := strings.TrimSpace(values[0])
	if value == "" {
		return "", false
	}
	return value, true
}

func parseVehicleType(value string) domain.VehicleType {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(domain.VehicleTypeScooter):
		return domain.VehicleTypeScooter
	case string(domain.VehicleTypeBike):
		return domain.VehicleTypeBike
	case string(domain.VehicleTypeCar):
		return domain.VehicleTypeCar
	default:
		return ""
	}
}
