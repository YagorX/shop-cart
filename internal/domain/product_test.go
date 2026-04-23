package domain

import (
	"testing"
	"time"
)

// ── CartItem.Validate ─────────────────────────────────────────────────────

func TestCartItem_Validate_OK(t *testing.T) {
	item := CartItem{
		ProductID:            "prod-1",
		Quantity:             1,
		PriceSnapshotKopecks: 100,
		AddedAt:              time.Now(),
	}
	if err := item.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCartItem_Validate_EmptyProductID(t *testing.T) {
	item := CartItem{
		ProductID:            "",
		Quantity:             1,
		PriceSnapshotKopecks: 100,
	}
	if err := item.Validate(); err == nil {
		t.Fatal("expected error for empty product_id, got nil")
	}
}

func TestCartItem_Validate_ZeroQuantity(t *testing.T) {
	item := CartItem{
		ProductID:            "prod-1",
		Quantity:             0,
		PriceSnapshotKopecks: 100,
	}
	if err := item.Validate(); err == nil {
		t.Fatal("expected error for zero quantity, got nil")
	}
}

func TestCartItem_Validate_NegativeQuantity(t *testing.T) {
	item := CartItem{
		ProductID:            "prod-1",
		Quantity:             -5,
		PriceSnapshotKopecks: 100,
	}
	if err := item.Validate(); err == nil {
		t.Fatal("expected error for negative quantity, got nil")
	}
}

func TestCartItem_Validate_ZeroPrice(t *testing.T) {
	item := CartItem{
		ProductID:            "prod-1",
		Quantity:             1,
		PriceSnapshotKopecks: 0,
	}
	if err := item.Validate(); err == nil {
		t.Fatal("expected error for zero price, got nil")
	}
}

func TestCartItem_Validate_NegativePrice(t *testing.T) {
	item := CartItem{
		ProductID:            "prod-1",
		Quantity:             1,
		PriceSnapshotKopecks: -1,
	}
	if err := item.Validate(); err == nil {
		t.Fatal("expected error for negative price, got nil")
	}
}

// ── Cart.Validate ─────────────────────────────────────────────────────────

func TestCart_Validate_OK(t *testing.T) {
	cart := Cart{UserID: "user-1"}
	if err := cart.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCart_Validate_EmptyUserID(t *testing.T) {
	cart := Cart{UserID: ""}
	if err := cart.Validate(); err == nil {
		t.Fatal("expected error for empty user_id, got nil")
	}
}
