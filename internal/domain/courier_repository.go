package domain

import "time"

// CourierRepository описывает хранилище курьеров и их операционных атрибутов.
type CourierRepository interface {
	// Create сохраняет нового курьера.
	Create(courier Courier) error
	// Get возвращает курьера по ID.
	Get(id string) (Courier, error)
	// GetByPhone возвращает курьера по уникальному номеру телефона.
	GetByPhone(phone string) (Courier, error)
	// Save обновляет профиль существующего курьера.
	Save(courier Courier) error
	// ListByZone возвращает курьеров, работающих в указанной зоне.
	ListByZone(zoneID string, limit int) ([]Courier, error)
	// ReplaceZones перезаписывает список зон курьера.
	ReplaceZones(courierID string, zones []CourierZone) error
	// ListZones возвращает все зоны, назначенные курьеру.
	ListZones(courierID string) ([]CourierZone, error)
	// CreateSlot сохраняет рабочий слот курьера.
	CreateSlot(slot CourierSlot) error
	// ListSlots возвращает слоты курьера за указанный интервал.
	ListSlots(courierID string, from, to time.Time) ([]CourierSlot, error)
}
