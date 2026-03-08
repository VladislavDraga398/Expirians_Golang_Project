package domain

import "errors"

var (
	// Ошибка отсутствующего идентификатора клиента.
	ErrCustomerRequired = errors.New("customer_id is required")
	// Ошибка отсутствующего кода валюты.
	ErrCurrencyRequired = errors.New("currency is required")
	// Ошибка отсутствия хотя бы одного товара в заказе.
	ErrItemsRequired = errors.New("order must contain at least one item")
	// Ошибка отрицательной суммы заказа.
	ErrAmountNegative = errors.New("amount_minor must be non-negative")
	// Ошибка при некорректном количестве товара (<= 0).
	ErrItemQtyInvalid = errors.New("item qty must be greater than zero")
	// Ошибка, если цена позиции отрицательная.
	ErrItemPriceInvalid = errors.New("item price must be non-negative")
	// Ошибка несоответствия суммы заказа и сумм позиций.
	ErrAmountMismatch = errors.New("order amount does not match items sum")
	// Ошибка отрицательной суммы платежа.
	ErrPaymentAmountNegative = errors.New("payment amount must be non-negative")
	// Ошибка отсутствующего кода платёжного провайдера.
	ErrPaymentProviderRequired = errors.New("payment provider is required")
	// Ошибка отсутствующего идентификатора заказа в платежах/резервах.
	ErrOrderIDRequired = errors.New("order_id is required")
	// Ошибка отсутствующего SKU в резерве.
	ErrReservationSKURequired = errors.New("reservation sku is required")
	// Ошибка некорректного количества в резерве.
	ErrReservationQtyInvalid = errors.New("reservation qty must be greater than zero")
	// ErrOrderNotFound возвращается, если заказ не найден в репозитории.
	ErrOrderNotFound = errors.New("order not found")
	// ErrOrderVersionConflict сигнализирует о конфликте версий при сохранении.
	ErrOrderVersionConflict = errors.New("order version conflict")
	// ErrInventoryUnavailable — бизнес-ошибка от склада (нет стока/недоступность позиции).
	ErrInventoryUnavailable = errors.New("inventory unavailable")
	// ErrInventoryTemporary — временная ошибка при обращении к складу, можно повторить попытку.
	ErrInventoryTemporary = errors.New("inventory temporary error")
	// ErrPaymentDeclined — платёж отклонён провайдером (бизнес-ошибка).
	ErrPaymentDeclined = errors.New("payment declined")
	// ErrPaymentIndeterminate — неопределённый статус платежа; требуется reconcile.
	ErrPaymentIndeterminate = errors.New("payment indeterminate state")
	// ErrPaymentTemporary — временная ошибка платёжного провайдера.
	ErrPaymentTemporary = errors.New("payment temporary error")
	// ErrOutboxPublish — ошибка при публикации сообщения из outbox.
	ErrOutboxPublish = errors.New("outbox publish failed")
	// ErrIdempotencyKeyRequired — отсутствует обязательный idempotency-key.
	ErrIdempotencyKeyRequired = errors.New("idempotency key is required")
	// ErrIdempotencyRequestHashRequired — отсутствует hash тела запроса для проверки replays.
	ErrIdempotencyRequestHashRequired = errors.New("idempotency request hash is required")
	// ErrIdempotencyKeyNotFound — ключ идемпотентности не найден.
	ErrIdempotencyKeyNotFound = errors.New("idempotency key not found")
	// ErrIdempotencyKeyAlreadyExists — ключ уже создан ранее.
	ErrIdempotencyKeyAlreadyExists = errors.New("idempotency key already exists")
	// ErrIdempotencyHashMismatch — ключ переиспользован с другим телом запроса.
	ErrIdempotencyHashMismatch = errors.New("idempotency request hash mismatch")
	// ErrCourierIDRequired — отсутствует идентификатор курьера.
	ErrCourierIDRequired = errors.New("courier id is required")
	// ErrCourierPhoneRequired — отсутствует номер телефона курьера.
	ErrCourierPhoneRequired = errors.New("courier phone is required")
	// ErrCourierPhoneFormatInvalid — номер телефона не прошёл форматную валидацию.
	ErrCourierPhoneFormatInvalid = errors.New("courier phone format is invalid")
	// ErrCourierFirstNameRequired — отсутствует имя курьера.
	ErrCourierFirstNameRequired = errors.New("courier first name is required")
	// ErrCourierLastNameRequired — отсутствует фамилия курьера.
	ErrCourierLastNameRequired = errors.New("courier last name is required")
	// ErrCourierVehicleTypeInvalid — указан неподдерживаемый тип транспорта.
	ErrCourierVehicleTypeInvalid = errors.New("courier vehicle type is invalid")
	// ErrCourierNotFound — курьер не найден в репозитории.
	ErrCourierNotFound = errors.New("courier not found")
	// ErrCourierAlreadyExists — курьер с таким ID уже существует.
	ErrCourierAlreadyExists = errors.New("courier already exists")
	// ErrCourierPhoneAlreadyExists — номер телефона уже привязан к другому курьеру.
	ErrCourierPhoneAlreadyExists = errors.New("courier phone already exists")
	// ErrCourierZoneRequired — не указан идентификатор зоны курьера.
	ErrCourierZoneRequired = errors.New("courier zone is required")
	// ErrCourierZoneUnknown — зона не найдена в справочнике Москвы.
	ErrCourierZoneUnknown = errors.New("courier zone is unknown")
	// ErrCourierZoneLimitExceeded — превышено допустимое количество зон для типа транспорта.
	ErrCourierZoneLimitExceeded = errors.New("courier zone limit exceeded")
	// ErrCourierZonesRequired — курьеру должна быть назначена хотя бы одна зона.
	ErrCourierZonesRequired = errors.New("courier zones are required")
	// ErrCourierZoneDuplicate — повторяющаяся зона в одном запросе назначения.
	ErrCourierZoneDuplicate = errors.New("courier zone duplicate")
	// ErrCourierPrimaryZoneConflict — в запросе указано более одной primary-зоны.
	ErrCourierPrimaryZoneConflict = errors.New("courier primary zone conflict")
	// ErrCourierZoneCapacityExceeded — превышен лимит курьеров по зоне.
	ErrCourierZoneCapacityExceeded = errors.New("courier zone capacity exceeded")
	// ErrCourierSlotIDRequired — отсутствует идентификатор слота.
	ErrCourierSlotIDRequired = errors.New("courier slot id is required")
	// ErrCourierSlotDurationInvalid — длительность слота отличается от поддерживаемых значений.
	ErrCourierSlotDurationInvalid = errors.New("courier slot duration is invalid")
	// ErrCourierSlotDurationMismatch — временной интервал слота не совпадает с duration_hours.
	ErrCourierSlotDurationMismatch = errors.New("courier slot duration mismatch")
	// ErrCourierSlotRangeInvalid — временной диапазон слота некорректен.
	ErrCourierSlotRangeInvalid = errors.New("courier slot range is invalid")
	// ErrCourierSlotStatusInvalid — статус слота курьера не поддерживается.
	ErrCourierSlotStatusInvalid = errors.New("courier slot status is invalid")
	// ErrCourierSlotConflict — слот пересекается с существующим или дублируется.
	ErrCourierSlotConflict = errors.New("courier slot conflict")
	// ErrCourierRatingIDRequired — отсутствует идентификатор оценки.
	ErrCourierRatingIDRequired = errors.New("courier rating id is required")
	// ErrCourierRatingScoreInvalid — оценка курьера вне допустимого диапазона.
	ErrCourierRatingScoreInvalid = errors.New("courier rating score is invalid")
	// ErrCourierRatingTagInvalid — передан неподдерживаемый тег оценки.
	ErrCourierRatingTagInvalid = errors.New("courier rating tag is invalid")
	// ErrCourierRatingTagDuplicate — в оценке передан повторяющийся тег.
	ErrCourierRatingTagDuplicate = errors.New("courier rating tag duplicate")
	// ErrCourierRatingReasonsRequired — для низкой оценки не указаны причины.
	ErrCourierRatingReasonsRequired = errors.New("courier rating reasons are required")
	// ErrCourierRatingPositiveTagsOnly — для 5 звёзд разрешены только позитивные теги.
	ErrCourierRatingPositiveTagsOnly = errors.New("courier rating positive tags only")
	// ErrCourierRatingAlreadyExists — оценка с таким идентификатором уже существует.
	ErrCourierRatingAlreadyExists = errors.New("courier rating already exists")
)

// IsVersionConflict проверяет, является ли ошибка конфликтом версий.
func IsVersionConflict(err error) bool {
	return errors.Is(err, ErrOrderVersionConflict)
}

// IsIdempotencyConflict возвращает true для коллизий ключа/запроса.
func IsIdempotencyConflict(err error) bool {
	return errors.Is(err, ErrIdempotencyKeyAlreadyExists) || errors.Is(err, ErrIdempotencyHashMismatch)
}
