package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

type courierRepository struct {
	db *sql.DB
}

// NewCourierRepository создаёт PostgreSQL-реализацию CourierRepository.
func NewCourierRepository(store *Store) domain.CourierRepository {
	return &courierRepository{db: store.DB()}
}

func (r *courierRepository) Create(courier domain.Courier) error {
	if err := firstDomainValidationErr(courier.ValidateInvariants()); err != nil {
		return err
	}
	normalizedPhone, err := domain.NormalizePhone(courier.Phone)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	now := time.Now().UTC()
	if courier.CreatedAt.IsZero() {
		courier.CreatedAt = now
	}
	if courier.UpdatedAt.IsZero() {
		courier.UpdatedAt = courier.CreatedAt
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO couriers (
			id, phone, first_name, last_name, vehicle_type, is_active, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`,
		strings.TrimSpace(courier.ID),
		normalizedPhone,
		strings.TrimSpace(courier.FirstName),
		strings.TrimSpace(courier.LastName),
		string(courier.VehicleType),
		courier.IsActive,
		courier.CreatedAt,
		courier.UpdatedAt,
	)
	if err != nil {
		return mapCourierUniqueErr(err)
	}

	return nil
}

func (r *courierRepository) Get(id string) (domain.Courier, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Courier{}, domain.ErrCourierIDRequired
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	var courier domain.Courier
	var vehicleType string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, phone, first_name, last_name, vehicle_type, is_active, created_at, updated_at
		FROM couriers
		WHERE id = $1
	`, id).Scan(
		&courier.ID,
		&courier.Phone,
		&courier.FirstName,
		&courier.LastName,
		&vehicleType,
		&courier.IsActive,
		&courier.CreatedAt,
		&courier.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Courier{}, domain.ErrCourierNotFound
		}
		return domain.Courier{}, fmt.Errorf("get courier: %w", err)
	}

	courier.VehicleType = domain.VehicleType(vehicleType)
	if !courier.VehicleType.Valid() {
		return domain.Courier{}, domain.ErrCourierVehicleTypeInvalid
	}

	return courier, nil
}

func (r *courierRepository) GetByPhone(phone string) (domain.Courier, error) {
	normalizedPhone, err := domain.NormalizePhone(phone)
	if err != nil {
		return domain.Courier{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	var courier domain.Courier
	var vehicleType string
	err = r.db.QueryRowContext(ctx, `
		SELECT id, phone, first_name, last_name, vehicle_type, is_active, created_at, updated_at
		FROM couriers
		WHERE phone = $1
	`, normalizedPhone).Scan(
		&courier.ID,
		&courier.Phone,
		&courier.FirstName,
		&courier.LastName,
		&vehicleType,
		&courier.IsActive,
		&courier.CreatedAt,
		&courier.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Courier{}, domain.ErrCourierNotFound
		}
		return domain.Courier{}, fmt.Errorf("get courier by phone: %w", err)
	}

	courier.VehicleType = domain.VehicleType(vehicleType)
	if !courier.VehicleType.Valid() {
		return domain.Courier{}, domain.ErrCourierVehicleTypeInvalid
	}

	return courier, nil
}

func (r *courierRepository) Save(courier domain.Courier) error {
	if err := firstDomainValidationErr(courier.ValidateInvariants()); err != nil {
		return err
	}
	normalizedPhone, err := domain.NormalizePhone(courier.Phone)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	if courier.UpdatedAt.IsZero() {
		courier.UpdatedAt = time.Now().UTC()
	}

	res, err := r.db.ExecContext(ctx, `
		UPDATE couriers
		SET phone = $1,
		    first_name = $2,
		    last_name = $3,
		    vehicle_type = $4,
		    is_active = $5,
		    updated_at = $6
		WHERE id = $7
	`,
		normalizedPhone,
		strings.TrimSpace(courier.FirstName),
		strings.TrimSpace(courier.LastName),
		string(courier.VehicleType),
		courier.IsActive,
		courier.UpdatedAt,
		strings.TrimSpace(courier.ID),
	)
	if err != nil {
		return mapCourierUniqueErr(err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("courier rows affected: %w", err)
	}
	if affected == 0 {
		return domain.ErrCourierNotFound
	}

	return nil
}

func (r *courierRepository) ListByZone(zoneID string, limit int) ([]domain.Courier, error) {
	zoneID = strings.TrimSpace(zoneID)
	if zoneID == "" {
		return nil, domain.ErrCourierZoneRequired
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	query := `
		SELECT c.id, c.phone, c.first_name, c.last_name, c.vehicle_type, c.is_active, c.created_at, c.updated_at
		FROM couriers AS c
		INNER JOIN courier_zones AS z ON z.courier_id = c.id
		WHERE z.zone_id = $1
		ORDER BY c.created_at DESC, c.id DESC
	`

	var (
		rows *sql.Rows
		err  error
	)
	if limit > 0 {
		rows, err = r.db.QueryContext(ctx, query+" LIMIT $2", zoneID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, query, zoneID)
	}
	if err != nil {
		return nil, fmt.Errorf("list couriers by zone: %w", err)
	}
	defer rows.Close()

	result := make([]domain.Courier, 0)
	for rows.Next() {
		var (
			courier     domain.Courier
			vehicleType string
		)
		if err := rows.Scan(
			&courier.ID,
			&courier.Phone,
			&courier.FirstName,
			&courier.LastName,
			&vehicleType,
			&courier.IsActive,
			&courier.CreatedAt,
			&courier.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan courier row: %w", err)
		}
		courier.VehicleType = domain.VehicleType(vehicleType)
		if !courier.VehicleType.Valid() {
			return nil, domain.ErrCourierVehicleTypeInvalid
		}
		result = append(result, courier)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate courier rows: %w", err)
	}

	return result, nil
}

func (r *courierRepository) ReplaceZones(courierID string, zones []domain.CourierZone) error {
	courierID = strings.TrimSpace(courierID)
	if courierID == "" {
		return domain.ErrCourierIDRequired
	}
	if len(zones) == 0 {
		return domain.ErrCourierZonesRequired
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin courier zones tx: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	courier, err := r.getCourierTx(ctx, tx, courierID)
	if err != nil {
		return err
	}

	if !courier.VehicleType.AllowsMultipleZones() && len(zones) > 1 {
		return domain.ErrCourierZoneLimitExceeded
	}

	prepared, err := prepareZones(courierID, zones)
	if err != nil {
		return err
	}
	if err := ensureZoneCapacityTx(ctx, tx, courierID, prepared); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM courier_zones WHERE courier_id = $1`, courierID); err != nil {
		return fmt.Errorf("delete courier zones: %w", err)
	}

	for _, zone := range prepared {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO courier_zones (courier_id, zone_id, is_primary, created_at)
			VALUES ($1,$2,$3,$4)
		`, zone.CourierID, zone.ZoneID, zone.IsPrimary, zone.AssignedAt); err != nil {
			return fmt.Errorf("insert courier zone: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit courier zones tx: %w", err)
	}
	committed = true

	return nil
}

func (r *courierRepository) ListZones(courierID string) ([]domain.CourierZone, error) {
	courierID = strings.TrimSpace(courierID)
	if courierID == "" {
		return nil, domain.ErrCourierIDRequired
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	if err := r.ensureCourierExists(ctx, courierID); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT courier_id, zone_id, is_primary, created_at
		FROM courier_zones
		WHERE courier_id = $1
		ORDER BY is_primary DESC, zone_id ASC
	`, courierID)
	if err != nil {
		return nil, fmt.Errorf("list courier zones: %w", err)
	}
	defer rows.Close()

	result := make([]domain.CourierZone, 0)
	for rows.Next() {
		var zone domain.CourierZone
		if err := rows.Scan(&zone.CourierID, &zone.ZoneID, &zone.IsPrimary, &zone.AssignedAt); err != nil {
			return nil, fmt.Errorf("scan courier zone: %w", err)
		}
		result = append(result, zone)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate courier zones: %w", err)
	}

	return result, nil
}

func (r *courierRepository) CreateSlot(slot domain.CourierSlot) error {
	if slot.Status == "" {
		slot.Status = domain.CourierSlotStatusPlanned
	}
	if err := firstDomainValidationErr(slot.ValidateInvariants()); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin courier slot tx: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := r.getCourierTx(ctx, tx, slot.CourierID); err != nil {
		return err
	}

	var overlapExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM courier_slots
			WHERE courier_id = $1
			  AND status <> 'canceled'
			  AND slot_start < $3
			  AND slot_end > $2
		)
	`, slot.CourierID, slot.SlotStart, slot.SlotEnd).Scan(&overlapExists); err != nil {
		return fmt.Errorf("check courier slot overlap: %w", err)
	}
	if overlapExists {
		return domain.ErrCourierSlotConflict
	}

	now := time.Now().UTC()
	if slot.CreatedAt.IsZero() {
		slot.CreatedAt = now
	}
	if slot.UpdatedAt.IsZero() {
		slot.UpdatedAt = slot.CreatedAt
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO courier_slots (
			id, courier_id, slot_start, slot_end, duration_hours, status, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`,
		slot.ID,
		slot.CourierID,
		slot.SlotStart,
		slot.SlotEnd,
		slot.DurationHours,
		string(slot.Status),
		slot.CreatedAt,
		slot.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrCourierSlotConflict
		}
		return fmt.Errorf("insert courier slot: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit courier slot tx: %w", err)
	}
	committed = true

	return nil
}

func (r *courierRepository) ListSlots(courierID string, from, to time.Time) ([]domain.CourierSlot, error) {
	courierID = strings.TrimSpace(courierID)
	if courierID == "" {
		return nil, domain.ErrCourierIDRequired
	}

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	if err := r.ensureCourierExists(ctx, courierID); err != nil {
		return nil, err
	}

	query := `
		SELECT id, courier_id, slot_start, slot_end, duration_hours, status, created_at, updated_at
		FROM courier_slots
		WHERE courier_id = $1
	`
	args := []any{courierID}

	switch {
	case !from.IsZero() && !to.IsZero():
		query += " AND slot_end > $2 AND slot_start < $3"
		args = append(args, from, to)
	case !from.IsZero():
		query += " AND slot_end > $2"
		args = append(args, from)
	case !to.IsZero():
		query += " AND slot_start < $2"
		args = append(args, to)
	}

	query += " ORDER BY slot_start ASC, id ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list courier slots: %w", err)
	}
	defer rows.Close()

	result := make([]domain.CourierSlot, 0)
	for rows.Next() {
		var (
			slot      domain.CourierSlot
			statusRaw string
		)
		if err := rows.Scan(
			&slot.ID,
			&slot.CourierID,
			&slot.SlotStart,
			&slot.SlotEnd,
			&slot.DurationHours,
			&statusRaw,
			&slot.CreatedAt,
			&slot.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan courier slot: %w", err)
		}
		slot.Status = domain.CourierSlotStatus(statusRaw)
		if !slot.Status.Valid() {
			return nil, domain.ErrCourierSlotStatusInvalid
		}
		result = append(result, slot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate courier slots: %w", err)
	}

	return result, nil
}

func firstDomainValidationErr(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return errs[0]
}

func prepareZones(courierID string, zones []domain.CourierZone) ([]domain.CourierZone, error) {
	seen := make(map[string]struct{}, len(zones))
	result := make([]domain.CourierZone, 0, len(zones))

	primaryCount := 0
	for _, zone := range zones {
		zone.CourierID = courierID
		zone.ZoneID = strings.TrimSpace(zone.ZoneID)
		if err := firstDomainValidationErr(zone.ValidateInvariants()); err != nil {
			return nil, err
		}
		if _, exists := seen[zone.ZoneID]; exists {
			return nil, domain.ErrCourierZoneDuplicate
		}
		seen[zone.ZoneID] = struct{}{}
		if zone.IsPrimary {
			primaryCount++
		}
		if zone.AssignedAt.IsZero() {
			zone.AssignedAt = time.Now().UTC()
		}
		result = append(result, zone)
	}

	if primaryCount > 1 {
		return nil, domain.ErrCourierPrimaryZoneConflict
	}
	if len(result) > 0 && primaryCount == 0 {
		result[0].IsPrimary = true
	}

	return result, nil
}

func ensureZoneCapacityTx(
	ctx context.Context,
	tx *sql.Tx,
	courierID string,
	zones []domain.CourierZone,
) error {
	for _, zone := range zones {
		var assignedCount int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT courier_id)
			FROM courier_zones
			WHERE zone_id = $1
			  AND courier_id <> $2
		`, zone.ZoneID, courierID).Scan(&assignedCount); err != nil {
			return fmt.Errorf("check zone capacity for zone %s: %w", zone.ZoneID, err)
		}
		if assignedCount >= domain.MaxCouriersPerZoneDefault {
			return domain.ErrCourierZoneCapacityExceeded
		}
	}

	return nil
}

func (r *courierRepository) ensureCourierExists(ctx context.Context, courierID string) error {
	var exists bool
	if err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM couriers
			WHERE id = $1
		)
	`, courierID).Scan(&exists); err != nil {
		return fmt.Errorf("check courier exists: %w", err)
	}
	if !exists {
		return domain.ErrCourierNotFound
	}
	return nil
}

func (r *courierRepository) getCourierTx(ctx context.Context, tx *sql.Tx, courierID string) (domain.Courier, error) {
	var (
		courier     domain.Courier
		vehicleType string
	)
	err := tx.QueryRowContext(ctx, `
		SELECT id, phone, first_name, last_name, vehicle_type, is_active, created_at, updated_at
		FROM couriers
		WHERE id = $1
		FOR UPDATE
	`, courierID).Scan(
		&courier.ID,
		&courier.Phone,
		&courier.FirstName,
		&courier.LastName,
		&vehicleType,
		&courier.IsActive,
		&courier.CreatedAt,
		&courier.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Courier{}, domain.ErrCourierNotFound
		}
		return domain.Courier{}, fmt.Errorf("get courier for update: %w", err)
	}

	courier.VehicleType = domain.VehicleType(vehicleType)
	if !courier.VehicleType.Valid() {
		return domain.Courier{}, domain.ErrCourierVehicleTypeInvalid
	}

	return courier, nil
}

func mapCourierUniqueErr(err error) error {
	if !isUniqueViolation(err) {
		return fmt.Errorf("courier query failed: %w", err)
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.ConstraintName == "couriers_phone_key" {
			return domain.ErrCourierPhoneAlreadyExists
		}
	}

	return domain.ErrCourierAlreadyExists
}

var _ domain.CourierRepository = (*courierRepository)(nil)
