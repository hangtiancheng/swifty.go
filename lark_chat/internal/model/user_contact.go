package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserContact struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	UserId      string             `bson:"user_id" json:"user_id"`
	ContactId   string             `bson:"contact_id" json:"contact_id"`
	ContactType int8               `bson:"contact_type" json:"contact_type"`
	Status      int8               `bson:"status" json:"status"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdateAt    time.Time          `bson:"update_at" json:"update_at"`
	DeletedAt   *time.Time         `bson:"deleted_at,omitempty" json:"-"`
}
