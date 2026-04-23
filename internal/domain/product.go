package domain

import (
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrCartNotFound = errors.New("cart not found")
	ErrItemNotFound = errors.New("item not found")
)

type CartItem struct {
	ProductID            string    `bson:"product_id"`
	Quantity             int32     `bson:"quantity"`
	PriceSnapshotKopecks int64     `bson:"price_snapshot_kopecks"`
	AddedAt              time.Time `bson:"added_at"`
}

type Cart struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    string             `bson:"user_id"`
	Items     []CartItem         `bson:"items"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

func (c *CartItem) Validate() error {
	if c.ProductID == "" {
		return errors.New("product_id is required")
	}
	if c.Quantity <= 0 {
		return errors.New("quantity must be greater than zero")
	}
	if c.PriceSnapshotKopecks <= 0 {
		return errors.New("price_snapshot_kopecks must be greater than zero")
	}
	return nil
}

func (c *Cart) Validate() error {
	if c.UserID == "" {
		return errors.New("user_id is required")
	}
	return nil
}
