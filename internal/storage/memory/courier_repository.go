package memory

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

type courierRepositoryInMemory struct {
	mu sync.RWMutex

	couriers       map[string]domain.Courier
	courierByPhone map[string]string
	zonesByCourier map[string]map[string]domain.CourierZone
	slotsByCourier map[string]map[string]domain.CourierSlot
}

// NewCourierRepository создаёт in-memory реализацию CourierRepository.
func NewCourierRepository() domain.CourierRepository {
	return &courierRepositoryInMemory{
		couriers:       make(map[string]domain.Courier),
		courierByPhone: make(map[string]string),
		zonesByCourier: make(map[string]map[string]domain.CourierZone),
		slotsByCourier: make(map[string]map[string]domain.CourierSlot),
	}
}

func (r *courierRepositoryInMemory) Create(courier domain.Courier) error {
	if err := firstValidationErr(courier.ValidateInvariants()); err != nil {
		return err
	}
	normalizedPhone, err := domain.NormalizePhone(courier.Phone)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.couriers[courier.ID]; exists {
		return domain.ErrCourierAlreadyExists
	}

	phone := normalizedPhone
	if existingID, exists := r.courierByPhone[phone]; exists && existingID != courier.ID {
		return domain.ErrCourierPhoneAlreadyExists
	}

	now := time.Now().UTC()
	if courier.CreatedAt.IsZero() {
		courier.CreatedAt = now
	}
	if courier.UpdatedAt.IsZero() {
		courier.UpdatedAt = courier.CreatedAt
	}

	courier.Phone = phone
	r.couriers[courier.ID] = cloneCourier(courier)
	r.courierByPhone[phone] = courier.ID
	return nil
}

func (r *courierRepositoryInMemory) Get(id string) (domain.Courier, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Courier{}, domain.ErrCourierIDRequired
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	courier, ok := r.couriers[id]
	if !ok {
		return domain.Courier{}, domain.ErrCourierNotFound
	}

	return cloneCourier(courier), nil
}

func (r *courierRepositoryInMemory) GetByPhone(phone string) (domain.Courier, error) {
	normalizedPhone, err := domain.NormalizePhone(phone)
	if err != nil {
		return domain.Courier{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	courierID, ok := r.courierByPhone[normalizedPhone]
	if !ok {
		return domain.Courier{}, domain.ErrCourierNotFound
	}

	courier, ok := r.couriers[courierID]
	if !ok {
		return domain.Courier{}, domain.ErrCourierNotFound
	}

	return cloneCourier(courier), nil
}

func (r *courierRepositoryInMemory) Save(courier domain.Courier) error {
	if err := firstValidationErr(courier.ValidateInvariants()); err != nil {
		return err
	}
	normalizedPhone, err := domain.NormalizePhone(courier.Phone)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.couriers[courier.ID]
	if !ok {
		return domain.ErrCourierNotFound
	}

	phone := normalizedPhone
	if existingID, exists := r.courierByPhone[phone]; exists && existingID != courier.ID {
		return domain.ErrCourierPhoneAlreadyExists
	}

	if currentPhone := strings.TrimSpace(current.Phone); currentPhone != phone {
		delete(r.courierByPhone, currentPhone)
		r.courierByPhone[phone] = courier.ID
	}

	if courier.CreatedAt.IsZero() {
		courier.CreatedAt = current.CreatedAt
	}
	if courier.UpdatedAt.IsZero() {
		courier.UpdatedAt = time.Now().UTC()
	}

	courier.Phone = phone
	r.couriers[courier.ID] = cloneCourier(courier)
	return nil
}

func (r *courierRepositoryInMemory) ListByZone(zoneID string, limit int) ([]domain.Courier, error) {
	zoneID = strings.TrimSpace(zoneID)
	if zoneID == "" {
		return nil, domain.ErrCourierZoneRequired
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.Courier, 0)
	for courierID, zones := range r.zonesByCourier {
		if _, ok := zones[zoneID]; !ok {
			continue
		}
		courier, exists := r.couriers[courierID]
		if !exists {
			continue
		}
		result = append(result, cloneCourier(courier))
	}

	sort.Slice(result, func(i, j int) bool {
		if !result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].CreatedAt.After(result[j].CreatedAt)
		}
		return result[i].ID > result[j].ID
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

func (r *courierRepositoryInMemory) ReplaceZones(courierID string, zones []domain.CourierZone) error {
	courierID = strings.TrimSpace(courierID)
	if courierID == "" {
		return domain.ErrCourierIDRequired
	}
	if len(zones) == 0 {
		return domain.ErrCourierZonesRequired
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	courier, exists := r.couriers[courierID]
	if !exists {
		return domain.ErrCourierNotFound
	}
	if !courier.VehicleType.AllowsMultipleZones() && len(zones) > 1 {
		return domain.ErrCourierZoneLimitExceeded
	}

	currentZones := r.zonesByCourier[courierID]
	seen := make(map[string]struct{}, len(zones))
	normalized := make([]domain.CourierZone, 0, len(zones))
	primaryCount := 0
	for _, zone := range zones {
		zone.CourierID = courierID
		if err := firstValidationErr(zone.ValidateInvariants()); err != nil {
			return err
		}
		zone.ZoneID = strings.TrimSpace(zone.ZoneID)
		if _, dup := seen[zone.ZoneID]; dup {
			return domain.ErrCourierZoneDuplicate
		}
		seen[zone.ZoneID] = struct{}{}
		if zone.IsPrimary {
			primaryCount++
		}
		if zone.AssignedAt.IsZero() {
			zone.AssignedAt = time.Now().UTC()
		}
		normalized = append(normalized, zone)
	}

	if primaryCount > 1 {
		return domain.ErrCourierPrimaryZoneConflict
	}
	if len(normalized) > 0 && primaryCount == 0 {
		normalized[0].IsPrimary = true
	}
	if err := r.ensureZoneCapacityLocked(courierID, normalized, currentZones); err != nil {
		return err
	}

	next := make(map[string]domain.CourierZone, len(normalized))
	for _, zone := range normalized {
		next[zone.ZoneID] = cloneCourierZone(zone)
	}
	r.zonesByCourier[courierID] = next

	return nil
}

func (r *courierRepositoryInMemory) ListZones(courierID string) ([]domain.CourierZone, error) {
	courierID = strings.TrimSpace(courierID)
	if courierID == "" {
		return nil, domain.ErrCourierIDRequired
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.couriers[courierID]; !exists {
		return nil, domain.ErrCourierNotFound
	}

	zonesMap := r.zonesByCourier[courierID]
	result := make([]domain.CourierZone, 0, len(zonesMap))
	for _, zone := range zonesMap {
		result = append(result, cloneCourierZone(zone))
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsPrimary != result[j].IsPrimary {
			return result[i].IsPrimary
		}
		return result[i].ZoneID < result[j].ZoneID
	})

	return result, nil
}

func (r *courierRepositoryInMemory) CreateSlot(slot domain.CourierSlot) error {
	if slot.Status == "" {
		slot.Status = domain.CourierSlotStatusPlanned
	}
	if err := firstValidationErr(slot.ValidateInvariants()); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.couriers[slot.CourierID]; !exists {
		return domain.ErrCourierNotFound
	}

	courierSlots, ok := r.slotsByCourier[slot.CourierID]
	if !ok {
		courierSlots = make(map[string]domain.CourierSlot)
		r.slotsByCourier[slot.CourierID] = courierSlots
	}
	if _, exists := courierSlots[slot.ID]; exists {
		return domain.ErrCourierSlotConflict
	}

	for _, existing := range courierSlots {
		if existing.Status == domain.CourierSlotStatusCanceled {
			continue
		}
		if intervalsOverlap(existing.SlotStart, existing.SlotEnd, slot.SlotStart, slot.SlotEnd) {
			return domain.ErrCourierSlotConflict
		}
	}

	now := time.Now().UTC()
	if slot.CreatedAt.IsZero() {
		slot.CreatedAt = now
	}
	if slot.UpdatedAt.IsZero() {
		slot.UpdatedAt = slot.CreatedAt
	}

	courierSlots[slot.ID] = cloneCourierSlot(slot)
	return nil
}

func (r *courierRepositoryInMemory) ListSlots(courierID string, from, to time.Time) ([]domain.CourierSlot, error) {
	courierID = strings.TrimSpace(courierID)
	if courierID == "" {
		return nil, domain.ErrCourierIDRequired
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.couriers[courierID]; !exists {
		return nil, domain.ErrCourierNotFound
	}

	slotsMap := r.slotsByCourier[courierID]
	result := make([]domain.CourierSlot, 0, len(slotsMap))
	for _, slot := range slotsMap {
		if !from.IsZero() && !slot.SlotEnd.After(from) {
			continue
		}
		if !to.IsZero() && !slot.SlotStart.Before(to) {
			continue
		}
		result = append(result, cloneCourierSlot(slot))
	}

	sort.Slice(result, func(i, j int) bool {
		if !result[i].SlotStart.Equal(result[j].SlotStart) {
			return result[i].SlotStart.Before(result[j].SlotStart)
		}
		return result[i].ID < result[j].ID
	})

	return result, nil
}

func firstValidationErr(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return errs[0]
}

func intervalsOverlap(startA, endA, startB, endB time.Time) bool {
	return startA.Before(endB) && startB.Before(endA)
}

func (r *courierRepositoryInMemory) ensureZoneCapacityLocked(
	courierID string,
	zones []domain.CourierZone,
	currentZones map[string]domain.CourierZone,
) error {
	for _, zone := range zones {
		// Если зона уже назначена этому курьеру, лимит не меняется.
		if _, exists := currentZones[zone.ZoneID]; exists {
			continue
		}
		count := r.countCouriersInZoneLocked(zone.ZoneID, courierID)
		if count >= domain.MaxCouriersPerZoneDefault {
			return domain.ErrCourierZoneCapacityExceeded
		}
	}
	return nil
}

func (r *courierRepositoryInMemory) countCouriersInZoneLocked(zoneID, excludeCourierID string) int {
	count := 0
	for courierID, zones := range r.zonesByCourier {
		if courierID == excludeCourierID {
			continue
		}
		if _, exists := zones[zoneID]; exists {
			count++
		}
	}
	return count
}

func cloneCourier(src domain.Courier) domain.Courier {
	return src
}

func cloneCourierZone(src domain.CourierZone) domain.CourierZone {
	return src
}

func cloneCourierSlot(src domain.CourierSlot) domain.CourierSlot {
	return src
}

var _ domain.CourierRepository = (*courierRepositoryInMemory)(nil)
