package domain

import (
	"strings"
	"time"
)

const (
	// MaxCouriersPerZoneDefault — базовый лимит активных назначений курьеров на одну зону.
	MaxCouriersPerZoneDefault = 100
	minPhoneDigits            = 11
	maxPhoneDigits            = 15
)

// VehicleType определяет транспорт курьера.
type VehicleType string

const (
	VehicleTypeScooter VehicleType = "scooter"
	VehicleTypeBike    VehicleType = "bike"
	VehicleTypeCar     VehicleType = "car"
)

// Valid проверяет, поддерживается ли тип транспорта.
func (v VehicleType) Valid() bool {
	switch v {
	case VehicleTypeScooter, VehicleTypeBike, VehicleTypeCar:
		return true
	default:
		return false
	}
}

// AllowsMultipleZones показывает, может ли курьер работать в нескольких зонах.
func (v VehicleType) AllowsMultipleZones() bool {
	return v == VehicleTypeCar
}

// Courier описывает профиль курьера в delivery-домене.
type Courier struct {
	ID          string
	Phone       string
	FirstName   string
	LastName    string
	VehicleType VehicleType
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ValidateInvariants проверяет инварианты профиля курьера.
func (c *Courier) ValidateInvariants() []error {
	var errs []error

	if strings.TrimSpace(c.ID) == "" {
		errs = append(errs, ErrCourierIDRequired)
	}
	if strings.TrimSpace(c.Phone) == "" {
		errs = append(errs, ErrCourierPhoneRequired)
	} else if _, err := NormalizePhone(c.Phone); err != nil {
		errs = append(errs, err)
	}
	if strings.TrimSpace(c.FirstName) == "" {
		errs = append(errs, ErrCourierFirstNameRequired)
	}
	if strings.TrimSpace(c.LastName) == "" {
		errs = append(errs, ErrCourierLastNameRequired)
	}
	if !c.VehicleType.Valid() {
		errs = append(errs, ErrCourierVehicleTypeInvalid)
	}

	return errs
}

// NormalizePhone приводит номер телефона к формату +<digits>.
func NormalizePhone(phone string) (string, error) {
	trimmed := strings.TrimSpace(phone)
	if trimmed == "" {
		return "", ErrCourierPhoneRequired
	}

	digits := make([]rune, 0, len(trimmed))
	for idx, r := range trimmed {
		switch {
		case r >= '0' && r <= '9':
			digits = append(digits, r)
		case r == '+' && idx == 0:
			// Разрешаем ведущий +, но в результате нормализуем независимо.
		case r == ' ' || r == '-' || r == '(' || r == ')' || r == '\t':
			// Игнорируем разделители.
		default:
			return "", ErrCourierPhoneFormatInvalid
		}
	}

	if len(digits) < minPhoneDigits || len(digits) > maxPhoneDigits {
		return "", ErrCourierPhoneFormatInvalid
	}

	return "+" + string(digits), nil
}

// CourierZone описывает привязку курьера к району/зоне.
type CourierZone struct {
	CourierID  string
	ZoneID     string
	IsPrimary  bool
	AssignedAt time.Time
}

// ValidateInvariants проверяет корректность привязки курьера к зоне.
func (z *CourierZone) ValidateInvariants() []error {
	var errs []error

	if strings.TrimSpace(z.CourierID) == "" {
		errs = append(errs, ErrCourierIDRequired)
	}
	if strings.TrimSpace(z.ZoneID) == "" {
		errs = append(errs, ErrCourierZoneRequired)
	}

	return errs
}

// CourierSlotStatus определяет статус рабочего слота курьера.
type CourierSlotStatus string

const (
	CourierSlotStatusPlanned   CourierSlotStatus = "planned"
	CourierSlotStatusActive    CourierSlotStatus = "active"
	CourierSlotStatusCompleted CourierSlotStatus = "completed"
	CourierSlotStatusCanceled  CourierSlotStatus = "canceled"
)

// Valid проверяет, поддерживается ли статус слота.
func (s CourierSlotStatus) Valid() bool {
	switch s {
	case CourierSlotStatusPlanned, CourierSlotStatusActive, CourierSlotStatusCompleted, CourierSlotStatusCanceled:
		return true
	default:
		return false
	}
}

// CourierSlot описывает рабочий слот курьера.
type CourierSlot struct {
	ID            string
	CourierID     string
	SlotStart     time.Time
	SlotEnd       time.Time
	DurationHours int
	Status        CourierSlotStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ValidateInvariants проверяет корректность слота курьера.
func (s *CourierSlot) ValidateInvariants() []error {
	var errs []error

	if strings.TrimSpace(s.ID) == "" {
		errs = append(errs, ErrCourierSlotIDRequired)
	}
	if strings.TrimSpace(s.CourierID) == "" {
		errs = append(errs, ErrCourierIDRequired)
	}
	if s.SlotStart.IsZero() || s.SlotEnd.IsZero() || !s.SlotEnd.After(s.SlotStart) {
		errs = append(errs, ErrCourierSlotRangeInvalid)
	}
	switch s.DurationHours {
	case 4, 8, 12:
	default:
		errs = append(errs, ErrCourierSlotDurationInvalid)
	}
	if !s.SlotStart.IsZero() && !s.SlotEnd.IsZero() && s.DurationHours > 0 {
		expected := time.Duration(s.DurationHours) * time.Hour
		if s.SlotEnd.Sub(s.SlotStart) != expected {
			errs = append(errs, ErrCourierSlotDurationMismatch)
		}
	}
	if s.Status == "" {
		// Пустой статус допустим до заполнения значением по умолчанию в сервисе/репозитории.
	} else if !s.Status.Valid() {
		errs = append(errs, ErrCourierSlotStatusInvalid)
	}

	return errs
}
