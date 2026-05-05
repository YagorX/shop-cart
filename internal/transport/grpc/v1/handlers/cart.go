package handlers

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/YagorX/shop-cart-service/internal/domain"
	"github.com/YagorX/shop-cart-service/internal/observability"
	cartsvc "github.com/YagorX/shop-cart-service/internal/service/cart"
	cartv1 "github.com/YagorX/shop-contracts/gen/go/proto/cart/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Handler struct {
	cartv1.UnimplementedCartServiceServer
	svc *cartsvc.CartService
}

func NewHandler(svc *cartsvc.CartService) (*Handler, error) {
	if svc == nil {
		return nil, errors.New("cart service is nil")
	}
	return &Handler{svc: svc}, nil
}

func userIDFromCtx(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}
	values := md.Get("user_id")
	if len(values) == 0 || values[0] == "" {
		return "", status.Error(codes.Unauthenticated, "missing user_id in metadata")
	}
	return values[0], nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrCartNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrItemNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func (h *Handler) AddItem(ctx context.Context, req *cartv1.AddItemRequest) (*cartv1.AddItemResponse, error) {
	const op = "transport.grpc.cart.AddItem"
	const method = "AddItem"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartGRPCRequestDuration.WithLabelValues(method).Observe(time.Since(startedAt).Seconds())
	}()

	userID, err := userIDFromCtx(ctx)
	if err != nil {
		metrics.CartGRPCRequestsTotal.WithLabelValues(method, codes.Unauthenticated.String()).Inc()
		return nil, err
	}

	item := domain.CartItem{
		ProductID:            req.GetProductId(),
		Quantity:             req.GetQuantity(),
		PriceSnapshotKopecks: req.GetPriceSnapshot(),
		AddedAt:              time.Now(),
	}

	slog.Debug("grpc AddItem",
		slog.String("op", op),
		slog.String("user_id", userID),
		slog.String("product_id", item.ProductID),
		slog.Int("quantity", int(item.Quantity)),
	)

	if err := h.svc.AddItem(ctx, userID, item); err != nil {
		mapped := mapError(err)
		metrics.CartGRPCRequestsTotal.WithLabelValues(method, status.Code(mapped).String()).Inc()
		slog.Error("grpc AddItem failed", slog.String("op", op), slog.String("error", err.Error()))
		return nil, mapped
	}

	metrics.CartGRPCRequestsTotal.WithLabelValues(method, codes.OK.String()).Inc()
	slog.Info("grpc AddItem ok", slog.String("op", op), slog.String("user_id", userID),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()))
	return &cartv1.AddItemResponse{}, nil
}

func (h *Handler) RemoveItem(ctx context.Context, req *cartv1.RemoveItemRequest) (*cartv1.RemoveItemResponse, error) {
	const op = "transport.grpc.cart.RemoveItem"
	const method = "RemoveItem"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartGRPCRequestDuration.WithLabelValues(method).Observe(time.Since(startedAt).Seconds())
	}()

	userID, err := userIDFromCtx(ctx)
	if err != nil {
		metrics.CartGRPCRequestsTotal.WithLabelValues(method, codes.Unauthenticated.String()).Inc()
		return nil, err
	}

	slog.Debug("grpc RemoveItem", slog.String("op", op), slog.String("user_id", userID),
		slog.String("product_id", req.GetProductId()))

	if err := h.svc.RemoveItem(ctx, userID, req.GetProductId()); err != nil {
		mapped := mapError(err)
		metrics.CartGRPCRequestsTotal.WithLabelValues(method, status.Code(mapped).String()).Inc()
		slog.Error("grpc RemoveItem failed", slog.String("op", op), slog.String("error", err.Error()))
		return nil, mapped
	}

	metrics.CartGRPCRequestsTotal.WithLabelValues(method, codes.OK.String()).Inc()
	slog.Info("grpc RemoveItem ok", slog.String("op", op), slog.String("user_id", userID),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()))
	return &cartv1.RemoveItemResponse{}, nil
}

func (h *Handler) UpdateItem(ctx context.Context, req *cartv1.UpdateItemRequest) (*cartv1.UpdateItemResponse, error) {
	const op = "transport.grpc.cart.UpdateItem"
	const method = "UpdateItem"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartGRPCRequestDuration.WithLabelValues(method).Observe(time.Since(startedAt).Seconds())
	}()

	userID, err := userIDFromCtx(ctx)
	if err != nil {
		metrics.CartGRPCRequestsTotal.WithLabelValues(method, codes.Unauthenticated.String()).Inc()
		return nil, err
	}

	slog.Debug("grpc UpdateItem", slog.String("op", op), slog.String("user_id", userID),
		slog.String("product_id", req.GetProductId()), slog.Int("quantity", int(req.GetQuantity())))

	if err := h.svc.UpdateItem(ctx, userID, req.GetProductId(), req.GetQuantity()); err != nil {
		mapped := mapError(err)
		metrics.CartGRPCRequestsTotal.WithLabelValues(method, status.Code(mapped).String()).Inc()
		slog.Error("grpc UpdateItem failed", slog.String("op", op), slog.String("error", err.Error()))
		return nil, mapped
	}

	metrics.CartGRPCRequestsTotal.WithLabelValues(method, codes.OK.String()).Inc()
	slog.Info("grpc UpdateItem ok", slog.String("op", op), slog.String("user_id", userID),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()))
	return &cartv1.UpdateItemResponse{}, nil
}

func (h *Handler) GetCart(ctx context.Context, req *cartv1.GetCartRequest) (*cartv1.GetCartResponse, error) {
	const op = "transport.grpc.cart.GetCart"
	const method = "GetCart"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartGRPCRequestDuration.WithLabelValues(method).Observe(time.Since(startedAt).Seconds())
	}()

	userID, err := userIDFromCtx(ctx)
	if err != nil {
		metrics.CartGRPCRequestsTotal.WithLabelValues(method, codes.Unauthenticated.String()).Inc()
		return nil, err
	}

	slog.Debug("grpc GetCart", slog.String("op", op), slog.String("user_id", userID))

	cart, err := h.svc.GetCart(ctx, userID)
	if err != nil {
		mapped := mapError(err)
		metrics.CartGRPCRequestsTotal.WithLabelValues(method, status.Code(mapped).String()).Inc()
		slog.Error("grpc GetCart failed", slog.String("op", op), slog.String("error", err.Error()))
		return nil, mapped
	}

	items := make([]*cartv1.CartItem, 0, len(cart.Items))
	for _, it := range cart.Items {
		items = append(items, &cartv1.CartItem{
			ProductId:     it.ProductID,
			Quantity:      it.Quantity,
			PriceSnapshot: it.PriceSnapshotKopecks,
			AddedAt:       timestamppb.New(it.AddedAt),
			Unavailable:   it.Unavailable,
		})
	}

	metrics.CartGRPCRequestsTotal.WithLabelValues(method, codes.OK.String()).Inc()
	slog.Info("grpc GetCart ok", slog.String("op", op), slog.String("user_id", userID),
		slog.Int("items", len(items)), slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()))
	return &cartv1.GetCartResponse{Items: items}, nil
}

func (h *Handler) ClearCart(ctx context.Context, req *cartv1.ClearCartRequest) (*cartv1.ClearCartResponse, error) {
	const op = "transport.grpc.cart.ClearCart"
	const method = "ClearCart"
	startedAt := time.Now()
	metrics := observability.MustMetrics()
	defer func() {
		metrics.CartGRPCRequestDuration.WithLabelValues(method).Observe(time.Since(startedAt).Seconds())
	}()

	userID, err := userIDFromCtx(ctx)
	if err != nil {
		metrics.CartGRPCRequestsTotal.WithLabelValues(method, codes.Unauthenticated.String()).Inc()
		return nil, err
	}

	slog.Debug("grpc ClearCart", slog.String("op", op), slog.String("user_id", userID))

	if err := h.svc.ClearCart(ctx, userID); err != nil {
		mapped := mapError(err)
		metrics.CartGRPCRequestsTotal.WithLabelValues(method, status.Code(mapped).String()).Inc()
		slog.Error("grpc ClearCart failed", slog.String("op", op), slog.String("error", err.Error()))
		return nil, mapped
	}

	metrics.CartGRPCRequestsTotal.WithLabelValues(method, codes.OK.String()).Inc()
	slog.Info("grpc ClearCart ok", slog.String("op", op), slog.String("user_id", userID),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()))
	return &cartv1.ClearCartResponse{}, nil
}
