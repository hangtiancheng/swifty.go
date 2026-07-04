package swifty_orm

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

func (q *Query) Distinct(ctx context.Context, field string) ([]interface{}, error) {
	if err := q.requireCollection(); err != nil {
		return nil, err
	}
	return q.collection.Distinct(ctx, field, q.buildFilter())
}

func (q *Query) Pluck(ctx context.Context, field string, out interface{}) error {
	q.fields = []string{field}
	return q.Find(ctx, out)
}

func (q *Query) Sum(ctx context.Context, field string) (float64, error) {
	return q.aggregate(ctx, bson.M{"$sum": "$" + field})
}

func (q *Query) Avg(ctx context.Context, field string) (float64, error) {
	return q.aggregate(ctx, bson.M{"$avg": "$" + field})
}

func (q *Query) Min(ctx context.Context, field string) (float64, error) {
	return q.aggregate(ctx, bson.M{"$min": "$" + field})
}

func (q *Query) Max(ctx context.Context, field string) (float64, error) {
	return q.aggregate(ctx, bson.M{"$max": "$" + field})
}

func (q *Query) aggregate(ctx context.Context, accumulator bson.M) (float64, error) {
	if err := q.requireCollection(); err != nil {
		return 0, err
	}
	pipeline := bson.A{
		bson.M{"$match": q.buildFilter()},
		bson.M{"$group": bson.M{"_id": nil, "result": accumulator}},
	}
	cursor, err := q.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	var results []struct {
		Result float64 `bson:"result"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return 0, err
	}
	if len(results) == 0 {
		return 0, nil
	}
	return results[0].Result, nil
}
