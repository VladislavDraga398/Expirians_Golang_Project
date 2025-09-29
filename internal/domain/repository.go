package domain

// OrderRepository описывает требования к хранилищу заказов.
type OrderRepository interface {
	// Create сохраняет новый заказ. Возвращает ошибку, если запись с таким ID уже существует.
	Create(order Order) error
	// Get возвращает заказ по идентификатору или ErrOrderNotFound, если его нет.
	Get(id string) (Order, error)
	// ListByCustomer возвращает заказы клиента с опциональным ограничением на количество.
	ListByCustomer(customerID string, limit int) ([]Order, error)
	// Save применяет обновления к заказу с учётом optimistic locking.
	Save(order Order) error
}
