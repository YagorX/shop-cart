package mongodb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/YagorX/shop-cart-service/internal/domain"
	"github.com/YagorX/shop-cart-service/internal/lock"
	"github.com/YagorX/shop-cart-service/internal/observability"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type cartRepository struct {
	collection      *mongo.Collection
	processedEvents *mongo.Collection
	lock            *lock.DistributedLock
}

func NewCartRepository(db *mongo.Database, lock *lock.DistributedLock) *cartRepository {
	return &cartRepository{
		collection:      db.Collection("carts"),
		processedEvents: db.Collection("processed_events"),
		lock:            lock,
	}
}

func (r *cartRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// для GetCart, AddItem, ClearCart
			Keys: bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetName("user_id_unique"),
		},
		{
			// для UpdateItem и RemoveItem (позиционный оператор)
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "items.product_id", Value: 1},
			},
			Options: options.Index().SetName("user_id_product_id"),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure indexes: %w", err)
	}

	_, err = r.processedEvents.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "processed_at", Value: 1}},
		Options: options.Index().SetName("processed_at"),
	})
	if err != nil {
		return fmt.Errorf("ensure processed events indexes: %w", err)
	}

	return nil
}

func (c *cartRepository) AddItem(ctx context.Context, userID string, item domain.CartItem) error {
	const op = "repository.mongodb.AddItem"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartMongoRequestDuration.WithLabelValues("AddItem").Observe(time.Since(startedAt).Seconds())
	}()

	lockValue, err := c.lock.Lock(ctx, userID)
	if err != nil {
		return fmt.Errorf("lock cart: %w", err)
	}
	defer c.lock.Unlock(ctx, userID, lockValue)

	slog.Debug("mongo AddItem", slog.String("op", op), slog.String("user_id", userID),
		slog.String("product_id", item.ProductID))

	filter := bson.D{
		{Key: "user_id", Value: userID},
		{Key: "items.product_id", Value: item.ProductID},
	}
	update := bson.D{
		{Key: "$inc", Value: bson.D{
			{Key: "items.$.quantity", Value: item.Quantity},
		}},
		{Key: "$set", Value: bson.D{
			{Key: "updated_at", Value: time.Now()},
		}},
	}

	result, err := c.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		metrics.CartMongoRequestsTotal.WithLabelValues("AddItem", "error").Inc()
		slog.Error("mongo AddItem inc failed", slog.String("op", op), slog.String("error", err.Error()))
		return err
	}

	if result.MatchedCount == 0 {
		filter2 := bson.D{{Key: "user_id", Value: userID}}
		update2 := bson.D{
			{Key: "$push", Value: bson.D{
				{Key: "items", Value: item},
			}},
			{Key: "$set", Value: bson.D{
				{Key: "updated_at", Value: time.Now()},
			}},
			{Key: "$setOnInsert", Value: bson.D{
				{Key: "created_at", Value: time.Now()},
			}},
		}

		opts := options.Update().SetUpsert(true)
		_, err = c.collection.UpdateOne(ctx, filter2, update2, opts)
		if err != nil {
			metrics.CartMongoRequestsTotal.WithLabelValues("AddItem", "error").Inc()
			slog.Error("mongo AddItem upsert failed", slog.String("op", op), slog.String("error", err.Error()))
			return err
		}
	}

	metrics.CartMongoRequestsTotal.WithLabelValues("AddItem", "ok").Inc()
	slog.Debug("mongo AddItem ok", slog.String("op", op),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()))
	return nil
}

func (c *cartRepository) RemoveItem(ctx context.Context, userID string, productID string) error {
	const op = "repository.mongodb.RemoveItem"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartMongoRequestDuration.WithLabelValues("RemoveItem").Observe(time.Since(startedAt).Seconds())
	}()

	lockValue, err := c.lock.Lock(ctx, userID)
	if err != nil {
		return fmt.Errorf("lock cart: %w", err)
	}
	defer c.lock.Unlock(ctx, userID, lockValue)

	slog.Debug("mongo RemoveItem", slog.String("op", op), slog.String("user_id", userID),
		slog.String("product_id", productID))

	filter := bson.D{{Key: "user_id", Value: userID}}
	update := bson.D{
		{Key: "$pull", Value: bson.D{
			{Key: "items", Value: bson.D{
				{Key: "product_id", Value: productID},
			}},
		}},
		{Key: "$set", Value: bson.D{
			{Key: "updated_at", Value: time.Now()},
		}},
	}

	result, err := c.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		metrics.CartMongoRequestsTotal.WithLabelValues("RemoveItem", "error").Inc()
		slog.Error("mongo RemoveItem failed", slog.String("op", op), slog.String("error", err.Error()))
		return err
	}
	if result.MatchedCount == 0 {
		metrics.CartMongoRequestsTotal.WithLabelValues("RemoveItem", "not_found").Inc()
		return domain.ErrCartNotFound
	}

	metrics.CartMongoRequestsTotal.WithLabelValues("RemoveItem", "ok").Inc()
	slog.Debug("mongo RemoveItem ok", slog.String("op", op),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()))
	return nil
}

func (c *cartRepository) UpdateItem(ctx context.Context, userID string, productID string, quantity int32) error {
	const op = "repository.mongodb.UpdateItem"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartMongoRequestDuration.WithLabelValues("UpdateItem").Observe(time.Since(startedAt).Seconds())
	}()

	lockValue, err := c.lock.Lock(ctx, userID)
	if err != nil {
		return fmt.Errorf("lock cart: %w", err)
	}
	defer c.lock.Unlock(ctx, userID, lockValue)

	slog.Debug("mongo UpdateItem", slog.String("op", op), slog.String("user_id", userID),
		slog.String("product_id", productID), slog.Int("quantity", int(quantity)))

	filter := bson.D{
		{Key: "user_id", Value: userID},
		{Key: "items.product_id", Value: productID},
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "items.$.quantity", Value: quantity},
			{Key: "updated_at", Value: time.Now()},
		}},
	}

	result, err := c.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		metrics.CartMongoRequestsTotal.WithLabelValues("UpdateItem", "error").Inc()
		slog.Error("mongo UpdateItem failed", slog.String("op", op), slog.String("error", err.Error()))
		return err
	}
	if result.MatchedCount == 0 {
		metrics.CartMongoRequestsTotal.WithLabelValues("UpdateItem", "not_found").Inc()
		return domain.ErrItemNotFound
	}

	metrics.CartMongoRequestsTotal.WithLabelValues("UpdateItem", "ok").Inc()
	slog.Debug("mongo UpdateItem ok", slog.String("op", op),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()))
	return nil
}

func (c *cartRepository) ClearCart(ctx context.Context, userID string) error {
	const op = "repository.mongodb.ClearCart"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartMongoRequestDuration.WithLabelValues("ClearCart").Observe(time.Since(startedAt).Seconds())
	}()

	lockValue, err := c.lock.Lock(ctx, userID)
	if err != nil {
		return fmt.Errorf("lock cart: %w", err)
	}
	defer c.lock.Unlock(ctx, userID, lockValue)

	slog.Debug("mongo ClearCart", slog.String("op", op), slog.String("user_id", userID))

	filter := bson.D{{Key: "user_id", Value: userID}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "items", Value: bson.A{}},
			{Key: "updated_at", Value: time.Now()},
		}},
	}

	result, err := c.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		metrics.CartMongoRequestsTotal.WithLabelValues("ClearCart", "error").Inc()
		slog.Error("mongo ClearCart failed", slog.String("op", op), slog.String("error", err.Error()))
		return err
	}
	if result.MatchedCount == 0 {
		metrics.CartMongoRequestsTotal.WithLabelValues("ClearCart", "not_found").Inc()
		return domain.ErrCartNotFound
	}

	metrics.CartMongoRequestsTotal.WithLabelValues("ClearCart", "ok").Inc()
	slog.Debug("mongo ClearCart ok", slog.String("op", op),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()))
	return nil
}

func (c *cartRepository) GetCart(ctx context.Context, userID string) (*domain.Cart, error) {
	const op = "repository.mongodb.GetCart"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartMongoRequestDuration.WithLabelValues("GetCart").Observe(time.Since(startedAt).Seconds())
	}()

	// GetCart - read операция, lock не нужен (для производительности)
	slog.Debug("mongo GetCart", slog.String("op", op), slog.String("user_id", userID))

	filter := bson.D{{Key: "user_id", Value: userID}}

	var cart domain.Cart
	err := c.collection.FindOne(ctx, filter).Decode(&cart)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			metrics.CartMongoRequestsTotal.WithLabelValues("GetCart", "not_found").Inc()
			return nil, domain.ErrCartNotFound
		}
		metrics.CartMongoRequestsTotal.WithLabelValues("GetCart", "error").Inc()
		slog.Error("mongo GetCart failed", slog.String("op", op), slog.String("error", err.Error()))
		return nil, err
	}

	metrics.CartMongoRequestsTotal.WithLabelValues("GetCart", "ok").Inc()
	slog.Debug("mongo GetCart ok", slog.String("op", op),
		slog.Int("items", len(cart.Items)),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()))
	return &cart, nil
}

// MarkItemUnavailableByProductID помечает товар как недоступный во ВСЕХ корзинах
// где он встречается. Используется когда stock товара стал 0.
func (c *cartRepository) MarkItemUnavailableByProductID(ctx context.Context, eventID string, productID string) (int64, error) {
	const op = "repository.mongodb.MarkItemUnavailableByProductID"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartMongoRequestDuration.WithLabelValues("MarkItemUnavailable").Observe(time.Since(startedAt).Seconds())
	}()

	slog.Debug("mongo MarkItemUnavailable", slog.String("op", op), slog.String("product_id", productID))

	filter := bson.D{
		{Key: "items.product_id", Value: productID},
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "items.$[item].unavailable", Value: true},
			{Key: "updated_at", Value: time.Now()},
		}},
	}

	// arrayFilters — обновляем только нужный элемент массива items
	opts := options.Update().SetArrayFilters(options.ArrayFilters{
		Filters: []interface{}{
			bson.D{{Key: "item.product_id", Value: productID}},
		},
	})

	session, err := c.collection.Database().Client().StartSession()
	if err != nil {
		return 0, fmt.Errorf("start mongo session: %w", err)
	}
	defer session.EndSession(ctx)

	var modifiedCount int64
	var matchedCount int64

	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		_, err := c.processedEvents.InsertOne(sc, bson.D{
			{Key: "_id", Value: eventID},
			{Key: "event_type", Value: "stock.changed"},
			{Key: "product_id", Value: productID},
			{Key: "processed_at", Value: time.Now()},
		})
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return nil, domain.ErrEventAlreadyProcessed
			}
			return nil, fmt.Errorf("insert processed event: %w", err)
		}

		result, err := c.collection.UpdateMany(sc, filter, update, opts)
		if err != nil {
			return nil, err
		}

		modifiedCount = result.ModifiedCount
		matchedCount = result.MatchedCount
		return nil, nil
	})
	if err != nil {
		if errors.Is(err, domain.ErrEventAlreadyProcessed) {
			slog.Info("catalog event already processed",
				slog.String("event_id", eventID),
				slog.String("product_id", productID),
			)
			return 0, nil
		}
		metrics.CartMongoRequestsTotal.WithLabelValues("MarkItemUnavailable", "error").Inc()
		slog.Error("mongo MarkItemUnavailable failed",
			slog.String("op", op),
			slog.String("event_id", eventID),
			slog.String("product_id", productID),
			slog.String("error", err.Error()),
		)
		return 0, err
	}

	metrics.CartMongoRequestsTotal.WithLabelValues("MarkItemUnavailable", "ok").Inc()
	slog.Info("mongo MarkItemUnavailable ok",
		slog.String("op", op),
		slog.String("event_id", eventID),
		slog.String("product_id", productID),
		slog.Int64("matched_count", matchedCount),
		slog.Int64("modified_count", modifiedCount),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)

	return modifiedCount, nil
}

// MarkItemAvailableByProductID — обратная операция, вызывается когда товар снова появился
func (c *cartRepository) MarkItemAvailableByProductID(ctx context.Context, eventID string, productID string) (int64, error) {
	const op = "repository.mongodb.MarkItemAvailableByProductID"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartMongoRequestDuration.WithLabelValues("MarkItemAvailable").Observe(time.Since(startedAt).Seconds())
	}()

	filter := bson.D{
		{Key: "items.product_id", Value: productID},
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "items.$[item].unavailable", Value: false},
			{Key: "updated_at", Value: time.Now()},
		}},
	}

	opts := options.Update().SetArrayFilters(options.ArrayFilters{
		Filters: []interface{}{
			bson.D{{Key: "item.product_id", Value: productID}},
		},
	})

	session, err := c.collection.Database().Client().StartSession()
	if err != nil {
		return 0, fmt.Errorf("start mongo session: %w", err)
	}
	defer session.EndSession(ctx)

	var modifiedCount int64
	var matchedCount int64

	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		_, err := c.processedEvents.InsertOne(sc, bson.D{
			{Key: "_id", Value: eventID},
			{Key: "event_type", Value: "stock.changed"},
			{Key: "product_id", Value: productID},
			{Key: "processed_at", Value: time.Now()},
		})
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return nil, domain.ErrEventAlreadyProcessed
			}
			return nil, fmt.Errorf("insert processed event: %w", err)
		}

		result, err := c.collection.UpdateMany(sc, filter, update, opts)
		if err != nil {
			return nil, err
		}

		modifiedCount = result.ModifiedCount
		matchedCount = result.MatchedCount
		return nil, nil
	})
	if err != nil {
		if errors.Is(err, domain.ErrEventAlreadyProcessed) {
			slog.Info("catalog event already processed",
				slog.String("event_id", eventID),
				slog.String("product_id", productID),
			)
			return 0, nil
		}
		metrics.CartMongoRequestsTotal.WithLabelValues("MarkItemAvailable", "error").Inc()
		slog.Error("mongo MarkItemAvailable failed",
			slog.String("op", op),
			slog.String("event_id", eventID),
			slog.String("product_id", productID),
			slog.String("error", err.Error()),
		)
		return 0, err
	}

	metrics.CartMongoRequestsTotal.WithLabelValues("MarkItemAvailable", "ok").Inc()
	slog.Info("mongo MarkItemAvailable ok",
		slog.String("op", op),
		slog.String("event_id", eventID),
		slog.String("product_id", productID),
		slog.Int64("matched_count", matchedCount),
		slog.Int64("modified_count", modifiedCount),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)

	return modifiedCount, nil
}
