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
	moscowUTCOffsetSeconds    = 3 * 60 * 60
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

// CourierVehicleCapability описывает грузовую способность типа транспорта курьера.
type CourierVehicleCapability struct {
	VehicleType      VehicleType
	MaxWeightGrams   int
	MaxVolumeCM3     int
	MaxOrdersPerTrip int
	UpdatedAt        time.Time
}

// ValidateInvariants проверяет корректность capability-профиля транспорта.
func (c *CourierVehicleCapability) ValidateInvariants() []error {
	var errs []error

	if !c.VehicleType.Valid() {
		errs = append(errs, ErrCourierVehicleTypeInvalid)
	}
	if c.MaxWeightGrams <= 0 || c.MaxVolumeCM3 <= 0 || c.MaxOrdersPerTrip <= 0 {
		errs = append(errs, ErrCourierVehicleCapabilityInvalid)
	}

	return errs
}

// DefaultCourierVehicleCapabilities возвращает базовые лимиты по типам транспорта.
func DefaultCourierVehicleCapabilities() []CourierVehicleCapability {
	return []CourierVehicleCapability{
		{
			VehicleType:      VehicleTypeScooter,
			MaxWeightGrams:   5000,
			MaxVolumeCM3:     35000,
			MaxOrdersPerTrip: 2,
		},
		{
			VehicleType:      VehicleTypeBike,
			MaxWeightGrams:   10000,
			MaxVolumeCM3:     65000,
			MaxOrdersPerTrip: 3,
		},
		{
			VehicleType:      VehicleTypeCar,
			MaxWeightGrams:   25000,
			MaxVolumeCM3:     250000,
			MaxOrdersPerTrip: 10,
		},
	}
}

// IsNightShiftSlot возвращает true, если слот соответствует окну 20:00-08:00 по Москве.
func IsNightShiftSlot(slotStart, slotEnd time.Time) bool {
	if slotStart.IsZero() || slotEnd.IsZero() {
		return false
	}

	msk := time.FixedZone("Europe/Moscow", moscowUTCOffsetSeconds)
	startMSK := slotStart.In(msk)
	endMSK := slotEnd.In(msk)

	if endMSK.Sub(startMSK) != 12*time.Hour {
		return false
	}

	return startMSK.Hour() == 20 &&
		startMSK.Minute() == 0 &&
		startMSK.Second() == 0 &&
		startMSK.Nanosecond() == 0 &&
		endMSK.Hour() == 8 &&
		endMSK.Minute() == 0 &&
		endMSK.Second() == 0 &&
		endMSK.Nanosecond() == 0
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
	zoneID := NormalizeZoneID(z.ZoneID)
	if zoneID == "" {
		errs = append(errs, ErrCourierZoneRequired)
	} else if !IsKnownMoscowZoneID(zoneID) {
		errs = append(errs, ErrCourierZoneUnknown)
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

const (
	// CourierRatingMinScore — минимально допустимая оценка курьера.
	CourierRatingMinScore = 1
	// CourierRatingMaxScore — максимально допустимая оценка курьера.
	CourierRatingMaxScore = 5
)

// CourierRatingTag определяет тег обратной связи по доставке.
type CourierRatingTag string

const (
	CourierRatingTagOnTime          CourierRatingTag = "on_time"
	CourierRatingTagPolite          CourierRatingTag = "polite"
	CourierRatingTagCarefulHandling CourierRatingTag = "careful_handling"
	CourierRatingTagDelayedDelivery CourierRatingTag = "delayed_delivery"
	CourierRatingTagRudeBehavior    CourierRatingTag = "rude_behavior"
	CourierRatingTagDamagedOrder    CourierRatingTag = "damaged_order"
	CourierRatingTagOtherIssue      CourierRatingTag = "other_issue"
)

// Valid проверяет, поддерживается ли тег рейтинга.
func (t CourierRatingTag) Valid() bool {
	switch t {
	case CourierRatingTagOnTime,
		CourierRatingTagPolite,
		CourierRatingTagCarefulHandling,
		CourierRatingTagDelayedDelivery,
		CourierRatingTagRudeBehavior,
		CourierRatingTagDamagedOrder,
		CourierRatingTagOtherIssue:
		return true
	default:
		return false
	}
}

// IsPositive возвращает true для позитивных тегов.
func (t CourierRatingTag) IsPositive() bool {
	switch t {
	case CourierRatingTagOnTime, CourierRatingTagPolite, CourierRatingTagCarefulHandling:
		return true
	default:
		return false
	}
}

// IsNegative возвращает true для негативных тегов.
func (t CourierRatingTag) IsNegative() bool {
	switch t {
	case CourierRatingTagDelayedDelivery, CourierRatingTagRudeBehavior, CourierRatingTagDamagedOrder, CourierRatingTagOtherIssue:
		return true
	default:
		return false
	}
}

// CourierRating описывает оценку доставки конкретного курьера.
type CourierRating struct {
	ID        string
	CourierID string
	Score     int
	Tags      []CourierRatingTag
	Comment   string
	CreatedAt time.Time
}

// ValidateInvariants проверяет корректность рейтинга курьера.
func (r *CourierRating) ValidateInvariants() []error {
	var errs []error

	if strings.TrimSpace(r.ID) == "" {
		errs = append(errs, ErrCourierRatingIDRequired)
	}
	if strings.TrimSpace(r.CourierID) == "" {
		errs = append(errs, ErrCourierIDRequired)
	}
	if r.Score < CourierRatingMinScore || r.Score > CourierRatingMaxScore {
		errs = append(errs, ErrCourierRatingScoreInvalid)
	}

	seen := make(map[CourierRatingTag]struct{}, len(r.Tags))
	hasNegativeTag := false
	for _, tag := range r.Tags {
		if !tag.Valid() {
			errs = append(errs, ErrCourierRatingTagInvalid)
			continue
		}
		if _, exists := seen[tag]; exists {
			errs = append(errs, ErrCourierRatingTagDuplicate)
			continue
		}
		seen[tag] = struct{}{}

		if tag.IsNegative() {
			hasNegativeTag = true
			if r.Score == CourierRatingMaxScore {
				errs = append(errs, ErrCourierRatingPositiveTagsOnly)
			}
		}
	}

	if r.Score < 3 && !hasNegativeTag {
		errs = append(errs, ErrCourierRatingReasonsRequired)
	}

	return errs
}

// CourierRatingSummary содержит агрегаты качества по курьеру.
type CourierRatingSummary struct {
	CourierID       string
	RatingsCount    int64
	AverageScore    float64
	LowRatingsCount int64
	Score1Count     int64
	Score2Count     int64
	Score3Count     int64
	Score4Count     int64
	Score5Count     int64
	OnTimeCount     int64
	PoliteCount     int64
	CarefulCount    int64
	DelayedCount    int64
	RudeCount       int64
	DamagedCount    int64
	OtherIssueCount int64
	LastRatingAt    time.Time
}
