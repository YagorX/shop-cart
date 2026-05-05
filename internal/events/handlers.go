package events

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

// CartUpdater — что handler умеет делать с корзинами
// Интерфейс здесь чтобы не зависеть от конкретной реализации mongo-репозитория
type CartUpdater interface {
	MarkItemUnavailableByProductID(ctx context.Context, eventID string, productID string) (int64, error)
	MarkItemAvailableByProductID(ctx context.Context, eventID string, productID string) (int64, error)
}

type EventHandler interface {
	Handle(ctx context.Context, msg kafka.Message) error
}

type catalogEventHandler struct {
	logger      *slog.Logger
	cartUpdater CartUpdater
}

func NewCatalogEventHandler(logger *slog.Logger, cartUpdater CartUpdater) EventHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &catalogEventHandler{
		logger:      logger,
		cartUpdater: cartUpdater,
	}
}

func (h *catalogEventHandler) Handle(ctx context.Context, msg kafka.Message) error {
	const op = "events.catalogEventHandler.Handle"

	var event ProductEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		// Битый JSON — нет смысла ретраить
		h.logger.Error("failed to unmarshal event",
			slog.String("op", op),
			slog.Int64("offset", msg.Offset),
			slog.String("error", err.Error()),
		)
		return nil // коммитим offset чтобы не зациклиться
	}

	if event.EventID == "" {
		h.logger.Error("event_id is empty",
			slog.String("op", op),
			slog.Int64("offset", msg.Offset),
		)
		return nil
	}

	h.logger.Info("catalog event received",
		slog.String("op", op),
		slog.String("event_type", event.EventType),
		slog.String("event_id", event.EventID),
		slog.String("product_id", event.ProductID),
		slog.Int("stock", int(event.Stock)),
		slog.Int64("offset", msg.Offset),
		slog.Int("partition", msg.Partition),
	)

	// Реагируем только на stock.changed — для cart-service остальные события неинтересны
	switch event.EventType {
	case EventStockChanged:
		return h.handleStockChanged(ctx, event)
	default:
		// product.created / product.updated — просто игнорируем
		// (можно добавить логику позже если появится бизнес-кейс)
		return nil
	}
}

func (h *catalogEventHandler) handleStockChanged(ctx context.Context, event ProductEvent) error {
	const op = "events.catalogEventHandler.handleStockChanged"

	if event.Stock > 0 {
		// Товар снова в наличии — снимаем флаг unavailable во всех корзинах
		modified, err := h.cartUpdater.MarkItemAvailableByProductID(ctx, event.EventID, event.ProductID)
		if err != nil {
			h.logger.Error("failed to mark item available",
				slog.String("op", op),
				slog.String("product_id", event.ProductID),
				slog.String("error", err.Error()),
			)
			return err // вернём ошибку → не коммитим → ретрай
		}
		if modified > 0 {
			h.logger.Info("items marked as available",
				slog.String("product_id", event.ProductID),
				slog.Int64("affected_carts", modified),
			)
		}
		return nil
	}

	// Stock == 0 — помечаем недоступным во всех корзинах
	modified, err := h.cartUpdater.MarkItemUnavailableByProductID(ctx, event.EventID, event.ProductID)
	if err != nil {
		h.logger.Error("failed to mark item unavailable",
			slog.String("op", op),
			slog.String("product_id", event.ProductID),
			slog.String("error", err.Error()),
		)
		return err
	}

	h.logger.Warn("product out of stock — marked unavailable in carts",
		slog.String("product_id", event.ProductID),
		slog.String("sku", event.SKU),
		slog.Int64("affected_carts", modified),
	)

	return nil
}
