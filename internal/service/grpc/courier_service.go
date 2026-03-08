package grpcsvc

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vladislavdragonenkov/oms/internal/domain"
	omsv1 "github.com/vladislavdragonenkov/oms/proto/oms/v1"
)

const defaultListCouriersByZoneLimit = 100

// CourierService реализует gRPC API для управления курьерами, зонами и слотами.
type CourierService struct {
	omsv1.UnimplementedCourierServiceServer

	repo   domain.CourierRepository
	logger *log.Entry
}

// NewCourierService конструирует CourierService с зависимостями.
func NewCourierService(repo domain.CourierRepository, logger *log.Entry) *CourierService {
	if logger == nil {
		logger = log.New().WithField("component", "courier-service")
	}

	return &CourierService{
		repo:   repo,
		logger: logger,
	}
}

// RegisterCourier регистрирует курьера и назначает стартовые зоны работы.
func (s *CourierService) RegisterCourier(_ context.Context, req *omsv1.RegisterCourierRequest) (*omsv1.RegisterCourierResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if s.repo == nil {
		return nil, status.Error(codes.Internal, "courier repository is not configured")
	}

	vehicleType, err := toDomainCourierVehicleType(req.VehicleType)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	courierID := strings.TrimSpace(req.CourierId)
	if courierID == "" {
		courierID = uuid.NewString()
	}

	zones, err := toDomainCourierZones(req.Zones, courierID, vehicleType)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	courier := domain.Courier{
		ID:          courierID,
		Phone:       req.Phone,
		FirstName:   strings.TrimSpace(req.FirstName),
		LastName:    strings.TrimSpace(req.LastName),
		VehicleType: vehicleType,
		IsActive:    true,
	}

	if err := s.repo.Create(courier); err != nil {
		return nil, s.mapCourierErr(err, "failed to create courier")
	}
	if err := s.repo.ReplaceZones(courierID, zones); err != nil {
		s.logger.WithField("courier_id", courierID).WithError(err).Warn("courier created but zone assignment failed")
		return nil, s.mapCourierErr(err, "failed to assign courier zones")
	}

	storedCourier, err := s.repo.Get(courierID)
	if err != nil {
		return nil, s.mapCourierErr(err, "failed to load courier after registration")
	}
	storedZones, err := s.repo.ListZones(courierID)
	if err != nil {
		return nil, s.mapCourierErr(err, "failed to load courier zones after registration")
	}

	return &omsv1.RegisterCourierResponse{
		Courier: toProtoCourier(storedCourier, storedZones),
	}, nil
}

// GetCourier возвращает профиль курьера вместе с текущими зонами.
func (s *CourierService) GetCourier(_ context.Context, req *omsv1.GetCourierRequest) (*omsv1.GetCourierResponse, error) {
	if req == nil || strings.TrimSpace(req.CourierId) == "" {
		return nil, status.Error(codes.InvalidArgument, "courier_id is required")
	}
	if s.repo == nil {
		return nil, status.Error(codes.Internal, "courier repository is not configured")
	}

	courier, err := s.repo.Get(req.CourierId)
	if err != nil {
		return nil, s.mapCourierErr(err, "failed to get courier")
	}
	zones, err := s.repo.ListZones(courier.ID)
	if err != nil {
		return nil, s.mapCourierErr(err, "failed to list courier zones")
	}

	return &omsv1.GetCourierResponse{
		Courier: toProtoCourier(courier, zones),
	}, nil
}

// ListCouriersByZone возвращает активных курьеров, назначенных в указанную зону.
func (s *CourierService) ListCouriersByZone(_ context.Context, req *omsv1.ListCouriersByZoneRequest) (*omsv1.ListCouriersByZoneResponse, error) {
	if req == nil || strings.TrimSpace(req.ZoneId) == "" {
		return nil, status.Error(codes.InvalidArgument, "zone_id is required")
	}
	if s.repo == nil {
		return nil, status.Error(codes.Internal, "courier repository is not configured")
	}
	if !domain.IsKnownMoscowZoneID(req.ZoneId) {
		return nil, status.Error(codes.InvalidArgument, domain.ErrCourierZoneUnknown.Error())
	}

	limit := int(req.Limit)
	if limit < 0 {
		return nil, status.Error(codes.InvalidArgument, "limit must be >= 0")
	}
	if limit == 0 {
		limit = defaultListCouriersByZoneLimit
	}

	couriers, err := s.repo.ListByZone(req.ZoneId, limit)
	if err != nil {
		return nil, s.mapCourierErr(err, "failed to list couriers by zone")
	}

	result := make([]*omsv1.Courier, 0, len(couriers))
	for _, courier := range couriers {
		zones, zoneErr := s.repo.ListZones(courier.ID)
		if zoneErr != nil {
			return nil, s.mapCourierErr(zoneErr, "failed to list courier zones")
		}
		result = append(result, toProtoCourier(courier, zones))
	}

	return &omsv1.ListCouriersByZoneResponse{Couriers: result}, nil
}

// ReplaceCourierZones полностью заменяет текущий список зон курьера.
func (s *CourierService) ReplaceCourierZones(_ context.Context, req *omsv1.ReplaceCourierZonesRequest) (*omsv1.ReplaceCourierZonesResponse, error) {
	if req == nil || strings.TrimSpace(req.CourierId) == "" {
		return nil, status.Error(codes.InvalidArgument, "courier_id is required")
	}
	if s.repo == nil {
		return nil, status.Error(codes.Internal, "courier repository is not configured")
	}

	courier, err := s.repo.Get(req.CourierId)
	if err != nil {
		return nil, s.mapCourierErr(err, "failed to get courier for zone replacement")
	}

	zones, err := toDomainCourierZones(req.Zones, courier.ID, courier.VehicleType)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.repo.ReplaceZones(courier.ID, zones); err != nil {
		return nil, s.mapCourierErr(err, "failed to replace courier zones")
	}

	storedZones, err := s.repo.ListZones(courier.ID)
	if err != nil {
		return nil, s.mapCourierErr(err, "failed to list courier zones after replacement")
	}

	return &omsv1.ReplaceCourierZonesResponse{
		CourierId: courier.ID,
		Zones:     toProtoCourierZones(storedZones),
	}, nil
}

// CreateCourierSlot создаёт рабочий слот курьера.
func (s *CourierService) CreateCourierSlot(_ context.Context, req *omsv1.CreateCourierSlotRequest) (*omsv1.CreateCourierSlotResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if s.repo == nil {
		return nil, status.Error(codes.Internal, "courier repository is not configured")
	}

	courierID := strings.TrimSpace(req.CourierId)
	if courierID == "" {
		return nil, status.Error(codes.InvalidArgument, "courier_id is required")
	}
	if req.SlotStartUnix <= 0 {
		return nil, status.Error(codes.InvalidArgument, "slot_start_unix is required")
	}
	if req.SlotEndUnix <= 0 {
		return nil, status.Error(codes.InvalidArgument, "slot_end_unix is required")
	}
	if req.DurationHours <= 0 {
		return nil, status.Error(codes.InvalidArgument, "duration_hours is required")
	}

	slotID := strings.TrimSpace(req.SlotId)
	if slotID == "" {
		slotID = uuid.NewString()
	}

	slot := domain.CourierSlot{
		ID:            slotID,
		CourierID:     courierID,
		SlotStart:     time.Unix(req.SlotStartUnix, 0).UTC(),
		SlotEnd:       time.Unix(req.SlotEndUnix, 0).UTC(),
		DurationHours: int(req.DurationHours),
		Status:        domain.CourierSlotStatusPlanned,
	}

	if err := s.repo.CreateSlot(slot); err != nil {
		return nil, s.mapCourierErr(err, "failed to create courier slot")
	}

	return &omsv1.CreateCourierSlotResponse{
		Slot: toProtoCourierSlot(slot),
	}, nil
}

// ListCourierSlots возвращает слоты курьера за указанный период.
func (s *CourierService) ListCourierSlots(_ context.Context, req *omsv1.ListCourierSlotsRequest) (*omsv1.ListCourierSlotsResponse, error) {
	if req == nil || strings.TrimSpace(req.CourierId) == "" {
		return nil, status.Error(codes.InvalidArgument, "courier_id is required")
	}
	if s.repo == nil {
		return nil, status.Error(codes.Internal, "courier repository is not configured")
	}
	if req.FromUnix < 0 || req.ToUnix < 0 {
		return nil, status.Error(codes.InvalidArgument, "from_unix/to_unix must be >= 0")
	}

	var from, to time.Time
	if req.FromUnix > 0 {
		from = time.Unix(req.FromUnix, 0).UTC()
	}
	if req.ToUnix > 0 {
		to = time.Unix(req.ToUnix, 0).UTC()
	}
	if !from.IsZero() && !to.IsZero() && !to.After(from) {
		return nil, status.Error(codes.InvalidArgument, "to_unix must be greater than from_unix")
	}

	slots, err := s.repo.ListSlots(req.CourierId, from, to)
	if err != nil {
		return nil, s.mapCourierErr(err, "failed to list courier slots")
	}

	result := make([]*omsv1.CourierSlot, 0, len(slots))
	for _, slot := range slots {
		result = append(result, toProtoCourierSlot(slot))
	}

	return &omsv1.ListCourierSlotsResponse{Slots: result}, nil
}

// GetCourierVehicleCapability возвращает capability-профиль указанного типа транспорта.
func (s *CourierService) GetCourierVehicleCapability(_ context.Context, req *omsv1.GetCourierVehicleCapabilityRequest) (*omsv1.GetCourierVehicleCapabilityResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if s.repo == nil {
		return nil, status.Error(codes.Internal, "courier repository is not configured")
	}

	vehicleType, err := toDomainCourierVehicleType(req.VehicleType)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	capability, err := s.repo.GetVehicleCapability(vehicleType)
	if err != nil {
		return nil, s.mapCourierErr(err, "failed to get courier vehicle capability")
	}

	return &omsv1.GetCourierVehicleCapabilityResponse{
		Capability: toProtoCourierVehicleCapability(capability),
	}, nil
}

// ListCourierVehicleCapabilities возвращает capability-профили по всем типам транспорта.
func (s *CourierService) ListCourierVehicleCapabilities(_ context.Context, _ *omsv1.ListCourierVehicleCapabilitiesRequest) (*omsv1.ListCourierVehicleCapabilitiesResponse, error) {
	if s.repo == nil {
		return nil, status.Error(codes.Internal, "courier repository is not configured")
	}

	capabilities, err := s.repo.ListVehicleCapabilities()
	if err != nil {
		return nil, s.mapCourierErr(err, "failed to list courier vehicle capabilities")
	}

	result := make([]*omsv1.CourierVehicleCapability, 0, len(capabilities))
	for _, capability := range capabilities {
		result = append(result, toProtoCourierVehicleCapability(capability))
	}

	return &omsv1.ListCourierVehicleCapabilitiesResponse{
		Capabilities: result,
	}, nil
}

// SubmitCourierRating сохраняет оценку качества доставки по курьеру.
func (s *CourierService) SubmitCourierRating(_ context.Context, req *omsv1.SubmitCourierRatingRequest) (*omsv1.SubmitCourierRatingResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if s.repo == nil {
		return nil, status.Error(codes.Internal, "courier repository is not configured")
	}

	courierID := strings.TrimSpace(req.CourierId)
	if courierID == "" {
		return nil, status.Error(codes.InvalidArgument, "courier_id is required")
	}

	ratingID := strings.TrimSpace(req.RatingId)
	if ratingID == "" {
		ratingID = uuid.NewString()
	}

	tags, err := toDomainCourierRatingTags(req.Tags)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	rating := domain.CourierRating{
		ID:        ratingID,
		CourierID: courierID,
		Score:     int(req.Score),
		Tags:      tags,
		Comment:   strings.TrimSpace(req.Comment),
	}
	if err := s.repo.SubmitRating(rating); err != nil {
		return nil, s.mapCourierErr(err, "failed to submit courier rating")
	}

	return &omsv1.SubmitCourierRatingResponse{
		RatingId:  ratingID,
		CourierId: courierID,
	}, nil
}

// GetCourierRatingSummary возвращает агрегированную сводку рейтингов курьера.
func (s *CourierService) GetCourierRatingSummary(_ context.Context, req *omsv1.GetCourierRatingSummaryRequest) (*omsv1.GetCourierRatingSummaryResponse, error) {
	if req == nil || strings.TrimSpace(req.CourierId) == "" {
		return nil, status.Error(codes.InvalidArgument, "courier_id is required")
	}
	if s.repo == nil {
		return nil, status.Error(codes.Internal, "courier repository is not configured")
	}

	summary, err := s.repo.GetRatingSummary(req.CourierId)
	if err != nil {
		return nil, s.mapCourierErr(err, "failed to get courier rating summary")
	}

	return &omsv1.GetCourierRatingSummaryResponse{
		Summary: toProtoCourierRatingSummary(summary),
	}, nil
}

func (s *CourierService) mapCourierErr(err error, internalMessage string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrCourierNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrCourierVehicleCapabilityNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrCourierAlreadyExists), errors.Is(err, domain.ErrCourierPhoneAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrCourierRatingAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrCourierZoneCapacityExceeded):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, domain.ErrCourierZoneLimitExceeded),
		errors.Is(err, domain.ErrCourierPrimaryZoneConflict),
		errors.Is(err, domain.ErrCourierNightSlotCarOnly),
		errors.Is(err, domain.ErrCourierSlotConflict):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrCourierIDRequired),
		errors.Is(err, domain.ErrCourierPhoneRequired),
		errors.Is(err, domain.ErrCourierPhoneFormatInvalid),
		errors.Is(err, domain.ErrCourierFirstNameRequired),
		errors.Is(err, domain.ErrCourierLastNameRequired),
		errors.Is(err, domain.ErrCourierVehicleTypeInvalid),
		errors.Is(err, domain.ErrCourierVehicleCapabilityInvalid),
		errors.Is(err, domain.ErrCourierZoneRequired),
		errors.Is(err, domain.ErrCourierZoneUnknown),
		errors.Is(err, domain.ErrCourierZonesRequired),
		errors.Is(err, domain.ErrCourierZoneDuplicate),
		errors.Is(err, domain.ErrCourierSlotIDRequired),
		errors.Is(err, domain.ErrCourierSlotDurationInvalid),
		errors.Is(err, domain.ErrCourierSlotDurationMismatch),
		errors.Is(err, domain.ErrCourierSlotRangeInvalid),
		errors.Is(err, domain.ErrCourierSlotStatusInvalid),
		errors.Is(err, domain.ErrCourierRatingIDRequired),
		errors.Is(err, domain.ErrCourierRatingScoreInvalid),
		errors.Is(err, domain.ErrCourierRatingTagInvalid),
		errors.Is(err, domain.ErrCourierRatingTagDuplicate),
		errors.Is(err, domain.ErrCourierRatingReasonsRequired),
		errors.Is(err, domain.ErrCourierRatingPositiveTagsOnly):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		s.logger.WithError(err).Error(internalMessage)
		return status.Error(codes.Internal, internalMessage)
	}
}

func toDomainCourierVehicleType(value omsv1.CourierVehicleType) (domain.VehicleType, error) {
	switch value {
	case omsv1.CourierVehicleType_COURIER_VEHICLE_TYPE_SCOOTER:
		return domain.VehicleTypeScooter, nil
	case omsv1.CourierVehicleType_COURIER_VEHICLE_TYPE_BIKE:
		return domain.VehicleTypeBike, nil
	case omsv1.CourierVehicleType_COURIER_VEHICLE_TYPE_CAR:
		return domain.VehicleTypeCar, nil
	default:
		return "", domain.ErrCourierVehicleTypeInvalid
	}
}

func toDomainCourierZones(inputs []*omsv1.CourierZoneInput, courierID string, vehicleType domain.VehicleType) ([]domain.CourierZone, error) {
	if len(inputs) == 0 {
		return nil, domain.ErrCourierZonesRequired
	}
	if !vehicleType.AllowsMultipleZones() && len(inputs) > 1 {
		return nil, domain.ErrCourierZoneLimitExceeded
	}

	seen := make(map[string]struct{}, len(inputs))
	zones := make([]domain.CourierZone, 0, len(inputs))
	primaryCount := 0
	for _, input := range inputs {
		if input == nil {
			return nil, domain.ErrCourierZoneRequired
		}

		zoneID := strings.TrimSpace(input.ZoneId)
		if zoneID == "" {
			return nil, domain.ErrCourierZoneRequired
		}
		zoneID = domain.NormalizeZoneID(zoneID)
		if !domain.IsKnownMoscowZoneID(zoneID) {
			return nil, domain.ErrCourierZoneUnknown
		}
		if _, exists := seen[zoneID]; exists {
			return nil, domain.ErrCourierZoneDuplicate
		}
		seen[zoneID] = struct{}{}

		if input.IsPrimary {
			primaryCount++
		}

		zones = append(zones, domain.CourierZone{
			CourierID: courierID,
			ZoneID:    zoneID,
			IsPrimary: input.IsPrimary,
		})
	}

	if primaryCount > 1 {
		return nil, domain.ErrCourierPrimaryZoneConflict
	}
	if primaryCount == 0 {
		zones[0].IsPrimary = true
	}

	return zones, nil
}

func toDomainCourierRatingTags(values []omsv1.CourierRatingTag) ([]domain.CourierRatingTag, error) {
	result := make([]domain.CourierRatingTag, 0, len(values))
	for _, value := range values {
		switch value {
		case omsv1.CourierRatingTag_COURIER_RATING_TAG_ON_TIME:
			result = append(result, domain.CourierRatingTagOnTime)
		case omsv1.CourierRatingTag_COURIER_RATING_TAG_POLITE:
			result = append(result, domain.CourierRatingTagPolite)
		case omsv1.CourierRatingTag_COURIER_RATING_TAG_CAREFUL_HANDLING:
			result = append(result, domain.CourierRatingTagCarefulHandling)
		case omsv1.CourierRatingTag_COURIER_RATING_TAG_DELAYED_DELIVERY:
			result = append(result, domain.CourierRatingTagDelayedDelivery)
		case omsv1.CourierRatingTag_COURIER_RATING_TAG_RUDE_BEHAVIOR:
			result = append(result, domain.CourierRatingTagRudeBehavior)
		case omsv1.CourierRatingTag_COURIER_RATING_TAG_DAMAGED_ORDER:
			result = append(result, domain.CourierRatingTagDamagedOrder)
		case omsv1.CourierRatingTag_COURIER_RATING_TAG_OTHER_ISSUE:
			result = append(result, domain.CourierRatingTagOtherIssue)
		default:
			return nil, domain.ErrCourierRatingTagInvalid
		}
	}
	return result, nil
}

func toProtoCourier(courier domain.Courier, zones []domain.CourierZone) *omsv1.Courier {
	return &omsv1.Courier{
		Id:          courier.ID,
		Phone:       courier.Phone,
		FirstName:   courier.FirstName,
		LastName:    courier.LastName,
		VehicleType: toProtoCourierVehicleType(courier.VehicleType),
		IsActive:    courier.IsActive,
		Zones:       toProtoCourierZones(zones),
	}
}

func toProtoCourierVehicleType(value domain.VehicleType) omsv1.CourierVehicleType {
	switch value {
	case domain.VehicleTypeScooter:
		return omsv1.CourierVehicleType_COURIER_VEHICLE_TYPE_SCOOTER
	case domain.VehicleTypeBike:
		return omsv1.CourierVehicleType_COURIER_VEHICLE_TYPE_BIKE
	case domain.VehicleTypeCar:
		return omsv1.CourierVehicleType_COURIER_VEHICLE_TYPE_CAR
	default:
		return omsv1.CourierVehicleType_COURIER_VEHICLE_TYPE_UNSPECIFIED
	}
}

func toProtoCourierZones(zones []domain.CourierZone) []*omsv1.CourierZone {
	result := make([]*omsv1.CourierZone, 0, len(zones))
	for _, zone := range zones {
		result = append(result, &omsv1.CourierZone{
			ZoneId:         zone.ZoneID,
			IsPrimary:      zone.IsPrimary,
			AssignedAtUnix: zone.AssignedAt.Unix(),
		})
	}
	return result
}

func toProtoCourierSlot(slot domain.CourierSlot) *omsv1.CourierSlot {
	return &omsv1.CourierSlot{
		Id:            slot.ID,
		CourierId:     slot.CourierID,
		SlotStartUnix: slot.SlotStart.Unix(),
		SlotEndUnix:   slot.SlotEnd.Unix(),
		DurationHours: toProtoInt32(slot.DurationHours),
		Status:        toProtoCourierSlotStatus(slot.Status),
	}
}

func toProtoCourierVehicleCapability(capability domain.CourierVehicleCapability) *omsv1.CourierVehicleCapability {
	return &omsv1.CourierVehicleCapability{
		VehicleType:      toProtoCourierVehicleType(capability.VehicleType),
		MaxWeightGrams:   toProtoInt32(capability.MaxWeightGrams),
		MaxVolumeCm3:     toProtoInt32(capability.MaxVolumeCM3),
		MaxOrdersPerTrip: toProtoInt32(capability.MaxOrdersPerTrip),
		UpdatedAtUnix:    capability.UpdatedAt.Unix(),
	}
}

func toProtoInt32(value int) int32 {
	if value > math.MaxInt32 {
		return math.MaxInt32
	}
	if value < math.MinInt32 {
		return math.MinInt32
	}
	return int32(value)
}

func toProtoCourierSlotStatus(value domain.CourierSlotStatus) omsv1.CourierSlotStatus {
	switch value {
	case domain.CourierSlotStatusPlanned:
		return omsv1.CourierSlotStatus_COURIER_SLOT_STATUS_PLANNED
	case domain.CourierSlotStatusActive:
		return omsv1.CourierSlotStatus_COURIER_SLOT_STATUS_ACTIVE
	case domain.CourierSlotStatusCompleted:
		return omsv1.CourierSlotStatus_COURIER_SLOT_STATUS_COMPLETED
	case domain.CourierSlotStatusCanceled:
		return omsv1.CourierSlotStatus_COURIER_SLOT_STATUS_CANCELED
	default:
		return omsv1.CourierSlotStatus_COURIER_SLOT_STATUS_UNSPECIFIED
	}
}

func toProtoCourierRatingSummary(summary domain.CourierRatingSummary) *omsv1.CourierRatingSummary {
	var lastRatingUnix int64
	if !summary.LastRatingAt.IsZero() {
		lastRatingUnix = summary.LastRatingAt.Unix()
	}

	return &omsv1.CourierRatingSummary{
		CourierId:       summary.CourierID,
		RatingsCount:    summary.RatingsCount,
		AverageScore:    summary.AverageScore,
		LowRatingsCount: summary.LowRatingsCount,
		Score_1Count:    summary.Score1Count,
		Score_2Count:    summary.Score2Count,
		Score_3Count:    summary.Score3Count,
		Score_4Count:    summary.Score4Count,
		Score_5Count:    summary.Score5Count,
		LastRatingUnix:  lastRatingUnix,
		OnTimeCount:     summary.OnTimeCount,
		PoliteCount:     summary.PoliteCount,
		CarefulCount:    summary.CarefulCount,
		DelayedCount:    summary.DelayedCount,
		RudeCount:       summary.RudeCount,
		DamagedCount:    summary.DamagedCount,
		OtherIssueCount: summary.OtherIssueCount,
	}
}

var _ omsv1.CourierServiceServer = (*CourierService)(nil)
