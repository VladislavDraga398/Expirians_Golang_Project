package pricing

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

func TestCalculator_Disabled(t *testing.T) {
	calc := NewCalculator(Config{
		Enabled:      false,
		BaseFeeMinor: 100,
	})

	result := calc.Calculate(context.Background())
	if result.Applied {
		t.Fatal("expected pricing to be disabled")
	}
	if result.DeliveryFeeMinor != 0 {
		t.Fatalf("expected fee=0, got %d", result.DeliveryFeeMinor)
	}
}

func TestCalculator_AppliesBaselineSignals(t *testing.T) {
	calc := NewCalculator(Config{
		Enabled:                true,
		BaseFeeMinor:           100,
		DefaultWeatherSeverity: 0.5,
		DefaultTrafficSeverity: 0.25,
		DefaultCourierLoad:     0.5,
		WeatherMaxBps:          2000,
		TrafficMaxBps:          1000,
		LoadMaxBps:             1000,
	})

	result := calc.Calculate(context.Background())
	if !result.Applied {
		t.Fatal("expected pricing to be applied")
	}
	// 100 * (1 + 0.10 + 0.025 + 0.05) = 117.5 => 118
	if result.DeliveryFeeMinor != 118 {
		t.Fatalf("expected fee=118, got %d", result.DeliveryFeeMinor)
	}
}

func TestCalculator_MetadataOverridesAndVehicleAwareFactors(t *testing.T) {
	calc := NewCalculator(Config{
		Enabled:                true,
		BaseFeeMinor:           100,
		DefaultWeatherSeverity: 0,
		DefaultTrafficSeverity: 0,
		DefaultCourierLoad:     0,
		WeatherMaxBps:          3000,
		TrafficMaxBps:          3000,
		LoadMaxBps:             2000,
	})

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		metadataVehicleType, string(domain.VehicleTypeCar),
		metadataWeatherSeverity, "1",
		metadataTrafficSeverity, "0.5",
		metadataCourierLoad, "0.25",
	))

	result := calc.Calculate(ctx)
	if !result.Applied {
		t.Fatal("expected pricing to be applied")
	}
	if result.WeatherBps != 0 {
		t.Fatalf("expected weatherBps=0 for car, got %d", result.WeatherBps)
	}
	if result.TrafficBps != 1500 {
		t.Fatalf("expected trafficBps=1500, got %d", result.TrafficBps)
	}
	if result.LoadBps != 500 {
		t.Fatalf("expected loadBps=500, got %d", result.LoadBps)
	}
	// 100 * (1 + 0.15 + 0.05) = 120
	if result.DeliveryFeeMinor != 120 {
		t.Fatalf("expected fee=120, got %d", result.DeliveryFeeMinor)
	}
}

func TestCalculator_ClampsMetadataOutOfRange(t *testing.T) {
	calc := NewCalculator(Config{
		Enabled:       true,
		BaseFeeMinor:  100,
		WeatherMaxBps: 1000,
		TrafficMaxBps: 1000,
		LoadMaxBps:    1000,
	})

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		metadataWeatherSeverity, "3",
		metadataTrafficSeverity, "-1",
		metadataCourierLoad, "2",
	))

	result := calc.Calculate(ctx)
	if result.WeatherSeverity != 1 {
		t.Fatalf("expected weatherSeverity clamp to 1, got %f", result.WeatherSeverity)
	}
	if result.TrafficSeverity != 0 {
		t.Fatalf("expected trafficSeverity clamp to 0, got %f", result.TrafficSeverity)
	}
	if result.CourierLoad != 1 {
		t.Fatalf("expected courierLoad clamp to 1, got %f", result.CourierLoad)
	}
}
