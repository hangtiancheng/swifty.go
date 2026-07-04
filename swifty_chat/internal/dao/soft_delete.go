package dao

import (
	"context"
	"time"

	"github.com/hangtiancheng/swifty.go/swifty_orm"
	"go.mongodb.org/mongo-driver/bson"
)

func ActiveQuery(model interface{}) *swifty_orm.Query {
	return Engine.Model(model).WhereNull("deleted_at")
}

func SoftDelete(ctx context.Context, model interface{}, field string, value interface{}) (int64, error) {
	return Engine.Model(model).Where(field, value).Update(ctx, bson.M{"deleted_at": time.Now()})
}
