package cart_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/YagorX/shop-cart-service/internal/domain"
	"github.com/YagorX/shop-cart-service/internal/service/cart"
)

// ── stub repository ───────────────────────────────────────────────────────────

type stubRepo struct {
	addItemFn    func(ctx context.Context, userID string, item domain.CartItem) error
	removeItemFn func(ctx context.Context, userID string, productID string) error
	updateItemFn func(ctx context.Context, userID string, productID string, quantity int32) error
	getCartFn    func(ctx context.Context, userID string) (*domain.Cart, error)
	clearCartFn  func(ctx context.Context, userID string) error
}

func (r *stubRepo) AddItem(ctx context.Context, userID string, item domain.CartItem) error {
	if r.addItemFn != nil {
		return r.addItemFn(ctx, userID, item)
	}
	return nil
}
func (r *stubRepo) RemoveItem(ctx context.Context, userID string, productID string) error {
	if r.removeItemFn != nil {
		return r.removeItemFn(ctx, userID, productID)
	}
	return nil
}
func (r *stubRepo) UpdateItem(ctx context.Context, userID string, productID string, quantity int32) error {
	if r.updateItemFn != nil {
		return r.updateItemFn(ctx, userID, productID, quantity)
	}
	return nil
}
func (r *stubRepo) GetCart(ctx context.Context, userID string) (*domain.Cart, error) {
	if r.getCartFn != nil {
		return r.getCartFn(ctx, userID)
	}
	return &domain.Cart{UserID: userID, Items: []domain.CartItem{}}, nil
}
func (r *stubRepo) ClearCart(ctx context.Context, userID string) error {
	if r.clearCartFn != nil {
		return r.clearCartFn(ctx, userID)
	}
	return nil
}

func newSilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(noopWriter{}, nil))
}

type noopWriter struct{}

func (noopWriter) Write(p []byte) (int, error) { return len(p), nil }

// ── helpers ───────────────────────────────────────────────────────────────────

func validItem() domain.CartItem {
	return domain.CartItem{
		ProductID:            "prod-1",
		Quantity:             2,
		PriceSnapshotKopecks: 500,
		AddedAt:              time.Now(),
	}
}

// ── AddItem ───────────────────────────────────────────────────────────────────

func TestCartService_AddItem_OK(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	if err := svc.AddItem(context.Background(), "user-1", validItem()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCartService_AddItem_EmptyUserID(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	err := svc.AddItem(context.Background(), "", validItem())
	if err == nil {
		t.Fatal("expected error for empty user_id, got nil")
	}
}

func TestCartService_AddItem_InvalidItem_EmptyProductID(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	item := validItem()
	item.ProductID = ""
	err := svc.AddItem(context.Background(), "user-1", item)
	if err == nil {
		t.Fatal("expected validation error for empty product_id, got nil")
	}
}

func TestCartService_AddItem_InvalidItem_ZeroQuantity(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	item := validItem()
	item.Quantity = 0
	if err := svc.AddItem(context.Background(), "user-1", item); err == nil {
		t.Fatal("expected error for zero quantity, got nil")
	}
}

func TestCartService_AddItem_InvalidItem_NegativePrice(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	item := validItem()
	item.PriceSnapshotKopecks = -1
	if err := svc.AddItem(context.Background(), "user-1", item); err == nil {
		t.Fatal("expected error for negative price, got nil")
	}
}

func TestCartService_AddItem_RepoError(t *testing.T) {
	repoErr := errors.New("mongo unavailable")
	stub := &stubRepo{
		addItemFn: func(_ context.Context, _ string, _ domain.CartItem) error {
			return repoErr
		},
	}
	svc := cart.NewCartService(stub, newSilentLogger())
	err := svc.AddItem(context.Background(), "user-1", validItem())
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected %v, got %v", repoErr, err)
	}
}

// ── RemoveItem ────────────────────────────────────────────────────────────────

func TestCartService_RemoveItem_OK(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	if err := svc.RemoveItem(context.Background(), "user-1", "prod-1"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCartService_RemoveItem_EmptyUserID(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	if err := svc.RemoveItem(context.Background(), "", "prod-1"); err == nil {
		t.Fatal("expected error for empty user_id, got nil")
	}
}

func TestCartService_RemoveItem_EmptyProductID(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	if err := svc.RemoveItem(context.Background(), "user-1", ""); err == nil {
		t.Fatal("expected error for empty product_id, got nil")
	}
}

func TestCartService_RemoveItem_RepoError(t *testing.T) {
	repoErr := errors.New("item not found")
	stub := &stubRepo{
		removeItemFn: func(_ context.Context, _ string, _ string) error {
			return repoErr
		},
	}
	svc := cart.NewCartService(stub, newSilentLogger())
	err := svc.RemoveItem(context.Background(), "user-1", "prod-1")
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected %v, got %v", repoErr, err)
	}
}

// ── UpdateItem ────────────────────────────────────────────────────────────────

func TestCartService_UpdateItem_OK(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	if err := svc.UpdateItem(context.Background(), "user-1", "prod-1", 5); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCartService_UpdateItem_EmptyUserID(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	if err := svc.UpdateItem(context.Background(), "", "prod-1", 5); err == nil {
		t.Fatal("expected error for empty user_id, got nil")
	}
}

func TestCartService_UpdateItem_EmptyProductID(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	if err := svc.UpdateItem(context.Background(), "user-1", "", 5); err == nil {
		t.Fatal("expected error for empty product_id, got nil")
	}
}

func TestCartService_UpdateItem_ZeroQuantity(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	if err := svc.UpdateItem(context.Background(), "user-1", "prod-1", 0); err == nil {
		t.Fatal("expected error for zero quantity, got nil")
	}
}

func TestCartService_UpdateItem_NegativeQuantity(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	if err := svc.UpdateItem(context.Background(), "user-1", "prod-1", -3); err == nil {
		t.Fatal("expected error for negative quantity, got nil")
	}
}

func TestCartService_UpdateItem_RepoError(t *testing.T) {
	repoErr := errors.New("update failed")
	stub := &stubRepo{
		updateItemFn: func(_ context.Context, _ string, _ string, _ int32) error {
			return repoErr
		},
	}
	svc := cart.NewCartService(stub, newSilentLogger())
	if err := svc.UpdateItem(context.Background(), "user-1", "prod-1", 3); !errors.Is(err, repoErr) {
		t.Fatalf("expected %v, got %v", repoErr, err)
	}
}

// ── GetCart ───────────────────────────────────────────────────────────────────

func TestCartService_GetCart_OK(t *testing.T) {
	expected := &domain.Cart{
		UserID: "user-1",
		Items: []domain.CartItem{
			{ProductID: "prod-1", Quantity: 2, PriceSnapshotKopecks: 300},
		},
	}
	stub := &stubRepo{
		getCartFn: func(_ context.Context, userID string) (*domain.Cart, error) {
			return expected, nil
		},
	}
	svc := cart.NewCartService(stub, newSilentLogger())
	got, err := svc.GetCart(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.UserID != expected.UserID {
		t.Fatalf("expected user_id %q, got %q", expected.UserID, got.UserID)
	}
	if len(got.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got.Items))
	}
}

func TestCartService_GetCart_EmptyUserID(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	if _, err := svc.GetCart(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty user_id, got nil")
	}
}

func TestCartService_GetCart_NotFound(t *testing.T) {
	stub := &stubRepo{
		getCartFn: func(_ context.Context, _ string) (*domain.Cart, error) {
			return nil, domain.ErrCartNotFound
		},
	}
	svc := cart.NewCartService(stub, newSilentLogger())
	_, err := svc.GetCart(context.Background(), "user-1")
	if !errors.Is(err, domain.ErrCartNotFound) {
		t.Fatalf("expected ErrCartNotFound, got %v", err)
	}
}

// ── ClearCart ─────────────────────────────────────────────────────────────────

func TestCartService_ClearCart_OK(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	if err := svc.ClearCart(context.Background(), "user-1"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCartService_ClearCart_EmptyUserID(t *testing.T) {
	svc := cart.NewCartService(&stubRepo{}, newSilentLogger())
	if err := svc.ClearCart(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty user_id, got nil")
	}
}

func TestCartService_ClearCart_RepoError(t *testing.T) {
	repoErr := errors.New("clear failed")
	stub := &stubRepo{
		clearCartFn: func(_ context.Context, _ string) error {
			return repoErr
		},
	}
	svc := cart.NewCartService(stub, newSilentLogger())
	if err := svc.ClearCart(context.Background(), "user-1"); !errors.Is(err, repoErr) {
		t.Fatalf("expected %v, got %v", repoErr, err)
	}
}
