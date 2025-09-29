package domain

import "time"

// TimelineEvent описывает событие в жизненном цикле заказа.
type TimelineEvent struct {
	OrderID  string
	Type     string
	Reason   string
	Occurred time.Time
}
