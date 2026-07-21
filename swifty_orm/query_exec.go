// Copyright (c) 2026 hangtiancheng
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package swifty_orm

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrCollectionRequired = errors.New("collection is required before query execution")

type InsertResult struct {
	InsertedIDs   []interface{}
	InsertedCount int64
}

func (q *Query) Insert(ctx context.Context, documents ...interface{}) (InsertResult, error) {
	if err := q.requireCollection(); err != nil {
		return InsertResult{}, err
	}
	if len(documents) == 0 {
		return InsertResult{}, errors.New("at least one document is required")
	}
	if len(documents) == 1 {
		result, err := q.collection.InsertOne(ctx, documents[0])
		if err != nil {
			return InsertResult{}, err
		}
		return InsertResult{InsertedIDs: []interface{}{result.InsertedID}, InsertedCount: 1}, nil
	}
	result, err := q.collection.InsertMany(ctx, documents)
	if err != nil {
		return InsertResult{}, err
	}
	return InsertResult{InsertedIDs: result.InsertedIDs, InsertedCount: int64(len(result.InsertedIDs))}, nil
}

func (q *Query) First(ctx context.Context, out interface{}) error {
	if err := q.requireCollection(); err != nil {
		return err
	}
	opts := options.FindOne()
	if len(q.sort) > 0 {
		opts.SetSort(q.sort)
	}
	if q.skip > 0 {
		opts.SetSkip(q.skip)
	}
	if proj := q.buildProjection(); proj != nil {
		opts.SetProjection(proj)
	}
	return q.collection.FindOne(ctx, q.buildFilter(), opts).Decode(out)
}

func (q *Query) Find(ctx context.Context, out interface{}) error {
	if err := q.requireCollection(); err != nil {
		return err
	}
	opts := options.Find()
	if len(q.sort) > 0 {
		opts.SetSort(q.sort)
	}
	if q.limit > 0 {
		opts.SetLimit(q.limit)
	}
	if q.skip > 0 {
		opts.SetSkip(q.skip)
	}
	if proj := q.buildProjection(); proj != nil {
		opts.SetProjection(proj)
	}
	cursor, err := q.collection.Find(ctx, q.buildFilter(), opts)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	return cursor.All(ctx, out)
}

func (q *Query) Update(ctx context.Context, update interface{}) (int64, error) {
	if err := q.requireCollection(); err != nil {
		return 0, err
	}
	result, err := q.collection.UpdateMany(ctx, q.buildFilter(), normalizeUpdate(update))
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (q *Query) Delete(ctx context.Context) (int64, error) {
	if err := q.requireCollection(); err != nil {
		return 0, err
	}
	result, err := q.collection.DeleteMany(ctx, q.buildFilter())
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

func (q *Query) Count(ctx context.Context) (int64, error) {
	if err := q.requireCollection(); err != nil {
		return 0, err
	}
	return q.collection.CountDocuments(ctx, q.buildFilter())
}

func (q *Query) Exists(ctx context.Context) (bool, error) {
	count, err := q.Count(ctx)
	return count > 0, err
}

func (q *Query) EnsureIndexes(ctx context.Context, indexes []mongo.IndexModel) ([]string, error) {
	if err := q.requireCollection(); err != nil {
		return nil, err
	}
	if len(indexes) == 0 {
		return nil, nil
	}
	return q.collection.Indexes().CreateMany(ctx, indexes)
}

func (q *Query) DropCollection(ctx context.Context) error {
	if err := q.requireCollection(); err != nil {
		return err
	}
	return q.collection.Drop(ctx)
}

func (q *Query) requireCollection() error {
	if q == nil || q.collection == nil {
		return ErrCollectionRequired
	}
	return nil
}

func normalizeUpdate(update interface{}) interface{} {
	doc, ok := update.(bson.M)
	if !ok {
		return update
	}
	for key := range doc {
		if strings.HasPrefix(key, "$") {
			return update
		}
	}
	return bson.M{"$set": doc}
}
