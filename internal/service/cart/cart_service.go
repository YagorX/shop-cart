package cart

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/YagorX/shop-cart-service/internal/domain"
	"github.com/YagorX/shop-cart-service/internal/observability"
)

type CartService struct {
	repo   CartRepository
	logger *slog.Logger
}

func NewCartService(repo CartRepository, logger *slog.Logger) *CartService {
	return &CartService{repo: repo, logger: logger}
}

type CartRepository interface {
	AddItem(ctx context.Context, userID string, item domain.CartItem) error
	RemoveItem(ctx context.Context, userID string, productID string) error
	UpdateItem(ctx context.Context, userID string, productID string, quantity int32) error
	GetCart(ctx context.Context, userID string) (*domain.Cart, error)
	ClearCart(ctx context.Context, userID string) error
}

func (s *CartService) AddItem(ctx context.Context, userID string, item domain.CartItem) error {
	const op = "service.cart.AddItem"
	const method = "AddItem"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartServiceRequestDuration.WithLabelValues(method).Observe(time.Since(startedAt).Seconds())
	}()

	if userID == "" {
		metrics.CartServiceRequestsTotal.WithLabelValues(method, "error").Inc()
		return errors.New("user_id is required")
	}
	if err := item.Validate(); err != nil {
		metrics.CartServiceRequestsTotal.WithLabelValues(method, "error").Inc()
		s.logger.Error("AddItem validation failed", slog.String("op", op), slog.String("error", err.Error()))
		return err
	}

	s.logger.Info("AddItem", slog.String("op", op), slog.String("user_id", userID),
		slog.String("product_id", item.ProductID))

	if err := s.repo.AddItem(ctx, userID, item); err != nil {
		metrics.CartServiceRequestsTotal.WithLabelValues(method, "error").Inc()
		s.logger.Error("AddItem failed", slog.String("op", op), slog.String("error", err.Error()))
		return err
	}

	metrics.CartServiceRequestsTotal.WithLabelValues(method, "ok").Inc()
	return nil
}

func (s *CartService) RemoveItem(ctx context.Context, userID string, productID string) error {
	const op = "service.cart.RemoveItem"
	const method = "RemoveItem"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartServiceRequestDuration.WithLabelValues(method).Observe(time.Since(startedAt).Seconds())
	}()

	if userID == "" {
		metrics.CartServiceRequestsTotal.WithLabelValues(method, "error").Inc()
		return errors.New("user_id is required")
	}
	if productID == "" {
		metrics.CartServiceRequestsTotal.WithLabelValues(method, "error").Inc()
		return errors.New("product_id is required")
	}

	s.logger.Info("RemoveItem", slog.String("op", op), slog.String("user_id", userID),
		slog.String("product_id", productID))

	if err := s.repo.RemoveItem(ctx, userID, productID); err != nil {
		metrics.CartServiceRequestsTotal.WithLabelValues(method, "error").Inc()
		s.logger.Error("RemoveItem failed", slog.String("op", op), slog.String("error", err.Error()))
		return err
	}

	metrics.CartServiceRequestsTotal.WithLabelValues(method, "ok").Inc()
	return nil
}

func (s *CartService) UpdateItem(ctx context.Context, userID string, productID string, quantity int32) error {
	const op = "service.cart.UpdateItem"
	const method = "UpdateItem"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartServiceRequestDuration.WithLabelValues(method).Observe(time.Since(startedAt).Seconds())
	}()

	if userID == "" {
		metrics.CartServiceRequestsTotal.WithLabelValues(method, "error").Inc()
		return errors.New("user_id is required")
	}
	if productID == "" {
		metrics.CartServiceRequestsTotal.WithLabelValues(method, "error").Inc()
		return errors.New("product_id is required")
	}
	if quantity <= 0 {
		metrics.CartServiceRequestsTotal.WithLabelValues(method, "error").Inc()
		return errors.New("quantity must be greater than zero")
	}

	s.logger.Info("UpdateItem", slog.String("op", op), slog.String("user_id", userID),
		slog.String("product_id", productID), slog.Int("quantity", int(quantity)))

	if err := s.repo.UpdateItem(ctx, userID, productID, quantity); err != nil {
		metrics.CartServiceRequestsTotal.WithLabelValues(method, "error").Inc()
		s.logger.Error("UpdateItem failed", slog.String("op", op), slog.String("error", err.Error()))
		return err
	}

	metrics.CartServiceRequestsTotal.WithLabelValues(method, "ok").Inc()
	return nil
}

func (s *CartService) GetCart(ctx context.Context, userID string) (*domain.Cart, error) {
	const op = "service.cart.GetCart"
	const method = "GetCart"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartServiceRequestDuration.WithLabelValues(method).Observe(time.Since(startedAt).Seconds())
	}()

	if userID == "" {
		metrics.CartServiceRequestsTotal.WithLabelValues(method, "error").Inc()
		return nil, errors.New("user_id is required")
	}

	s.logger.Info("GetCart", slog.String("op", op), slog.String("user_id", userID))

	cart, err := s.repo.GetCart(ctx, userID)
	if err != nil {
		metrics.CartServiceRequestsTotal.WithLabelValues(method, "error").Inc()
		s.logger.Error("GetCart failed", slog.String("op", op), slog.String("error", err.Error()))
		return nil, err
	}

	metrics.CartServiceRequestsTotal.WithLabelValues(method, "ok").Inc()
	return cart, nil
}

func (s *CartService) ClearCart(ctx context.Context, userID string) error {
	const op = "service.cart.ClearCart"
	const method = "ClearCart"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartServiceRequestDuration.WithLabelValues(method).Observe(time.Since(startedAt).Seconds())
	}()

	if userID == "" {
		metrics.CartServiceRequestsTotal.WithLabelValues(method, "error").Inc()
		return errors.New("user_id is required")
	}

	s.logger.Info("ClearCart", slog.String("op", op), slog.String("user_id", userID))

	if err := s.repo.ClearCart(ctx, userID); err != nil {
		metrics.CartServiceRequestsTotal.WithLabelValues(method, "error").Inc()
		s.logger.Error("ClearCart failed", slog.String("op", op), slog.String("error", err.Error()))
		return err
	}

	metrics.CartServiceRequestsTotal.WithLabelValues(method, "ok").Inc()
	return nil
}
