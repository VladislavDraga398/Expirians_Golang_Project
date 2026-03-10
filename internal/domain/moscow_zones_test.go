package domain_test

import (
	"testing"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

func TestIsKnownMoscowZoneID(t *testing.T) {
	if !domain.IsKnownMoscowZoneID("msk-cao-arbat") {
		t.Fatal("expected msk-cao-arbat to be known")
	}
	if !domain.IsKnownMoscowZoneID("  MSK-CAO-TVERSKOY  ") {
		t.Fatal("expected zone normalization to work for known zone")
	}
	if domain.IsKnownMoscowZoneID("msk-cao-unknown") {
		t.Fatal("expected unknown zone to be rejected")
	}
}

func TestMoscowZonesCatalogSortedAndNonEmpty(t *testing.T) {
	catalog := domain.MoscowZonesCatalog()
	if len(catalog) == 0 {
		t.Fatal("expected non-empty Moscow zones catalog")
	}

	for i := 1; i < len(catalog); i++ {
		if catalog[i-1].ID > catalog[i].ID {
			t.Fatalf("catalog must be sorted by id: %s > %s", catalog[i-1].ID, catalog[i].ID)
		}
	}
}
