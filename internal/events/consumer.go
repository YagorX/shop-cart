package events

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	EventProductCreated  = "product.created"
	EventProductUpdated  = "product.updated"
	EventStockChanged    = "stock.changed"
	topicCatalogProducts = "catalog.products.v1"
	consumerGroupID      = "cart-service-catalog-consumer"
)

// ProductEvent — единая структура для всех событий каталога
type ProductEvent struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	ProductID  string    `json:"product_id"`
	SKU        string    `json:"sku"`
	Name       string    `json:"name"`
	PriceCents int64     `json:"price_cents"`
	Currency   string    `json:"currency"`
	Stock      int32     `json:"stock"`
	Active     bool      `json:"active"`
	OccuredAt  time.Time `json:"occured_at"`
}

type CatalogEventConsumer struct {
	reader  *kafka.Reader
	handler EventHandler
	logger  *slog.Logger
}

func NewCatalogEventConsumer(brokers []string, handler EventHandler, logger *slog.Logger) *CatalogEventConsumer {
	if logger == nil {
		logger = slog.Default()
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       topicCatalogProducts,
		GroupID:     consumerGroupID,
		StartOffset: kafka.LastOffset,
		MaxWait:     500 * time.Millisecond,
		MinBytes:    1,
		MaxBytes:    10 * 1024 * 1024, // 10MB
		ErrorLogger: kafka.LoggerFunc(func(msg string, args ...interface{}) {
			logger.Error("kafka reader error",
				slog.String("msg", msg),
			)
		}),
	})

	return &CatalogEventConsumer{
		reader:  reader,
		handler: handler,
		logger:  logger,
	}
}

// Run запускает consumer loop. Блокирующий вызов — запускать в горутине.
// Возвращает nil при graceful shutdown (ctx.Done()), иначе ошибку.
func (c *CatalogEventConsumer) Run(ctx context.Context) error {
	const op = "events.CatalogEventConsumer.Run"

	c.logger.Info("catalog event consumer started",
		slog.String("op", op),
		slog.String("topic", topicCatalogProducts),
		slog.String("group_id", consumerGroupID),
	)

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				c.logger.Info("consumer stopping (context cancelled)", slog.String("op", op))
				return nil
			}
			// Сетевая ошибка — kafka-go сам ретраит, продолжаем
			c.logger.Error("fetch message failed",
				slog.String("op", op),
				slog.String("error", err.Error()),
			)
			time.Sleep(1 * time.Second)
			continue
		}

		// Обрабатываем
		if err := c.handler.Handle(ctx, msg); err != nil {
			c.logger.Error("handle message failed",
				slog.String("op", op),
				slog.Int64("offset", msg.Offset),
				slog.Int("partition", msg.Partition),
				slog.String("error", err.Error()),
			)
			// НЕ коммитим — следующий FetchMessage вернёт это же сообщение
			// В проде нужен retry с backoff и DLQ после N попыток
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Коммитим только после успешной обработки
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.Error("commit message failed",
				slog.String("op", op),
				slog.Int64("offset", msg.Offset),
				slog.String("error", err.Error()),
			)
			// Сообщение обработано, но коммит не прошёл
			// При рестарте обработаем повторно — нужна idempotency
			continue
		}
	}
}

// Close закрывает reader. Вызывать в Shutdown после отмены контекста.
func (c *CatalogEventConsumer) Close() error {
	if c.reader != nil {
		return c.reader.Close()
	}
	return nil
}
