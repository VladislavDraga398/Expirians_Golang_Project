package domain

import "time"

// IdempotencyStatus описывает жизненный цикл ключа идемпотентности.
type IdempotencyStatus string

const (
	// IdempotencyStatusProcessing означает, что запрос принят и ещё обрабатывается.
	IdempotencyStatusProcessing IdempotencyStatus = "processing"
	// IdempotencyStatusDone означает, что запрос завершён успешно и ответ сохранён.
	IdempotencyStatusDone IdempotencyStatus = "done"
	// IdempotencyStatusFailed означает, что обработка завершилась ошибкой.
	IdempotencyStatusFailed IdempotencyStatus = "failed"
)

// IdempotencyRecord хранит состояние обработки запроса с idempotency-key.
type IdempotencyRecord struct {
	Key          string
	RequestHash  string
	ResponseBody []byte
	HTTPStatus   int
	Status       IdempotencyStatus
	TTLAt        time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Valid проверяет, что статус относится к поддерживаемым значениям.
func (s IdempotencyStatus) Valid() bool {
	switch s {
	case IdempotencyStatusProcessing, IdempotencyStatusDone, IdempotencyStatusFailed:
		return true
	default:
		return false
	}
}
