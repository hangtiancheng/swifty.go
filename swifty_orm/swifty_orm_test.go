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
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const defaultTestMongoURI = "mongodb://localhost:27017/"

type testUser struct {
	ID        int64     `bson:"_id"`
	Name      string    `bson:"name"`
	Email     string    `bson:"email"`
	Age       int       `bson:"age"`
	CreatedAt time.Time `bson:"created_at"`
}

type testCity struct {
	Name string `bson:"name"`
}

func openTestEngine(t *testing.T) (*Engine, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = defaultTestMongoURI
	}
	database := fmt.Sprintf("swifty_orm_test_%d", time.Now().UnixNano())
	engine, err := NewEngine(ctx, uri, database)
	if err != nil {
		t.Fatalf("NewEngine returned error: %v", err)
	}
	if err := engine.Database().RunCommand(ctx, bson.D{{Key: "create", Value: "__access_check"}}).Err(); err != nil {
		_ = engine.Close(ctx)
		if isMongoUnauthorized(err) {
			t.Skipf("MongoDB requires authentication; set MONGO_URI with credentials: %v", err)
		}
		t.Fatalf("MongoDB access check returned error: %v", err)
	}
	_ = engine.Collection("__access_check").DropCollection(ctx)
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer dropCancel()
		if err := engine.DropDatabase(dropCtx); err != nil {
			if isMongoUnauthorized(err) {
				return
			}
			t.Fatalf("DropDatabase returned error: %v", err)
		}
		_ = engine.Close(dropCtx)
	})
	return engine, ctx
}

func isMongoUnauthorized(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Unauthorized")
}

func TestNewEngineValidation(t *testing.T) {
	ctx := context.Background()
	if _, err := NewEngine(ctx, "", "db"); err == nil {
		t.Fatal("expected error for empty uri")
	}
	if _, err := NewEngine(ctx, "mongodb://localhost:27017", ""); err == nil {
		t.Fatal("expected error for empty database")
	}
	var e *Engine
	if err := e.Close(ctx); err != nil {
		t.Fatalf("nil Close returned error: %v", err)
	}
	if e.Client() != nil || e.Database() != nil || e.DatabaseName() != "" {
		t.Fatal("nil engine accessors should return zero values")
	}
}

func TestCollectionName(t *testing.T) {
	cases := map[string]interface{}{
		"test_users":  &testUser{},
		"test_cities": []testCity{},
		"":            42,
	}
	for want, input := range cases {
		if got := CollectionName(input); got != want {
			t.Fatalf("CollectionName(%T) = %q, want %q", input, got, want)
		}
	}
}

func TestWhereInsertFindUpdateDeleteCount(t *testing.T) {
	engine, ctx := openTestEngine(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	_, err := engine.Collection("users").Insert(ctx,
		&testUser{ID: 1, Name: "Tom", Email: "tom@example.com", Age: 18, CreatedAt: now},
		&testUser{ID: 2, Name: "Sam", Email: "sam@example.com", Age: 20, CreatedAt: now},
		&testUser{ID: 3, Name: "Amy", Email: "amy@example.com", Age: 25, CreatedAt: now},
	)
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}

	count, err := engine.Collection("users").Count(ctx)
	if err != nil || count != 3 {
		t.Fatalf("Count = %d, %v", count, err)
	}

	var first testUser
	err = engine.Collection("users").
		Where("age", ">=", 18).
		OrderBy("age", "desc").
		First(ctx, &first)
	if err != nil || first.Name != "Amy" {
		t.Fatalf("First = %+v, %v", first, err)
	}

	var users []testUser
	err = engine.Collection("users").
		Where("age", ">=", 18).
		OrderBy("age").
		Limit(2).
		Offset(1).
		Find(ctx, &users)
	if err != nil || len(users) != 2 || users[0].Name != "Sam" {
		t.Fatalf("Find = %+v, %v", users, err)
	}

	modified, err := engine.Collection("users").
		Where("name", "Tom").
		Update(ctx, bson.M{"age": 19})
	if err != nil || modified != 1 {
		t.Fatalf("Update = %d, %v", modified, err)
	}

	deleted, err := engine.Collection("users").
		Where("age", ">=", 19).
		Delete(ctx)
	if err != nil || deleted != 3 {
		t.Fatalf("Delete = %d, %v", deleted, err)
	}
}

func TestWhereInAndBetween(t *testing.T) {
	engine, ctx := openTestEngine(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	engine.Collection("users").Insert(ctx,
		&testUser{ID: 1, Name: "Tom", Age: 18, CreatedAt: now},
		&testUser{ID: 2, Name: "Sam", Age: 25, CreatedAt: now},
		&testUser{ID: 3, Name: "Amy", Age: 30, CreatedAt: now},
	)

	var users []testUser
	err := engine.Collection("users").
		WhereIn("name", []string{"Tom", "Amy"}).
		OrderBy("age").
		Find(ctx, &users)
	if err != nil || len(users) != 2 || users[0].Name != "Tom" || users[1].Name != "Amy" {
		t.Fatalf("WhereIn = %+v, %v", users, err)
	}

	users = nil
	err = engine.Collection("users").
		WhereBetween("age", 20, 30).
		Find(ctx, &users)
	if err != nil || len(users) != 2 {
		t.Fatalf("WhereBetween = %+v, %v", users, err)
	}
}

func TestOrWhere(t *testing.T) {
	engine, ctx := openTestEngine(t)
	engine.Collection("users").Insert(ctx,
		&testUser{ID: 1, Name: "Tom", Age: 18},
		&testUser{ID: 2, Name: "Sam", Age: 25},
		&testUser{ID: 3, Name: "Amy", Age: 30},
	)

	var users []testUser
	err := engine.Collection("users").
		Where("name", "Tom").
		OrWhere("name", "Amy").
		OrderBy("age").
		Find(ctx, &users)
	if err != nil || len(users) != 2 {
		t.Fatalf("OrWhere = %+v, %v", users, err)
	}
}

func TestSelectProjection(t *testing.T) {
	engine, ctx := openTestEngine(t)
	engine.Collection("users").Insert(ctx, &testUser{ID: 1, Name: "Tom", Email: "tom@e.com", Age: 18})

	var users []testUser
	err := engine.Collection("users").
		Select("name", "_id").
		Find(ctx, &users)
	if err != nil || len(users) != 1 || users[0].Name != "Tom" || users[0].Email != "" {
		t.Fatalf("Select = %+v, %v", users, err)
	}
}

func TestExists(t *testing.T) {
	engine, ctx := openTestEngine(t)
	engine.Collection("users").Insert(ctx, &testUser{ID: 1, Name: "Tom"})

	exists, err := engine.Collection("users").Where("name", "Tom").Exists(ctx)
	if err != nil || !exists {
		t.Fatalf("Exists(Tom) = %v, %v", exists, err)
	}
	exists, err = engine.Collection("users").Where("name", "Nobody").Exists(ctx)
	if err != nil || exists {
		t.Fatalf("Exists(Nobody) = %v, %v", exists, err)
	}
}

func TestAggregation(t *testing.T) {
	engine, ctx := openTestEngine(t)
	engine.Collection("users").Insert(ctx,
		&testUser{ID: 1, Name: "Tom", Age: 10},
		&testUser{ID: 2, Name: "Sam", Age: 20},
		&testUser{ID: 3, Name: "Amy", Age: 30},
	)

	sum, err := engine.Collection("users").Sum(ctx, "age")
	if err != nil || sum != 60 {
		t.Fatalf("Sum = %v, %v", sum, err)
	}
	avg, err := engine.Collection("users").Avg(ctx, "age")
	if err != nil || avg != 20 {
		t.Fatalf("Avg = %v, %v", avg, err)
	}
	min, err := engine.Collection("users").Min(ctx, "age")
	if err != nil || min != 10 {
		t.Fatalf("Min = %v, %v", min, err)
	}
	max, err := engine.Collection("users").Max(ctx, "age")
	if err != nil || max != 30 {
		t.Fatalf("Max = %v, %v", max, err)
	}
}

func TestDistinct(t *testing.T) {
	engine, ctx := openTestEngine(t)
	engine.Collection("users").Insert(ctx,
		&testUser{ID: 1, Name: "Tom", Age: 18},
		&testUser{ID: 2, Name: "Sam", Age: 18},
		&testUser{ID: 3, Name: "Amy", Age: 25},
	)

	values, err := engine.Collection("users").Distinct(ctx, "age")
	if err != nil || len(values) != 2 {
		t.Fatalf("Distinct = %v, %v", values, err)
	}
}

func TestEnsureIndexesAndDrop(t *testing.T) {
	engine, ctx := openTestEngine(t)
	names, err := engine.Collection("users").EnsureIndexes(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_email")},
	})
	if err != nil || len(names) != 1 || names[0] != "uniq_email" {
		t.Fatalf("EnsureIndexes = %v, %v", names, err)
	}
	engine.Collection("users").Insert(ctx, &testUser{ID: 1, Email: "dupe@e.com"})
	if _, err := engine.Collection("users").Insert(ctx, &testUser{ID: 2, Email: "dupe@e.com"}); err == nil {
		t.Fatal("expected unique index error")
	}
	if err := engine.Collection("users").DropCollection(ctx); err != nil {
		t.Fatalf("DropCollection returned error: %v", err)
	}
	if err := engine.Collection("users").First(ctx, &testUser{}); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("First after drop = %v", err)
	}
}

func TestNextSequence(t *testing.T) {
	engine, ctx := openTestEngine(t)
	first, err := engine.NextSequence(ctx, "users")
	if err != nil || first != 1 {
		t.Fatalf("first = %d, %v", first, err)
	}
	second, err := engine.NextSequence(ctx, "users")
	if err != nil || second != 2 {
		t.Fatalf("second = %d, %v", second, err)
	}
	if _, err := engine.NextSequence(ctx, ""); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestQueryValidation(t *testing.T) {
	engine, ctx := openTestEngine(t)
	q := engine.Collection("")
	if _, err := q.Insert(ctx); err == nil {
		t.Fatal("expected collection required error")
	}
	if _, err := engine.Collection("test").Insert(ctx); err == nil {
		t.Fatal("expected empty insert error")
	}
}

func TestModelEntryPoint(t *testing.T) {
	engine, ctx := openTestEngine(t)
	_, err := engine.Model(&testUser{}).Insert(ctx, &testUser{ID: 1, Name: "model"})
	if err != nil {
		t.Fatalf("Model insert returned error: %v", err)
	}
	var out testUser
	err = engine.Model(&testUser{}).Where("name", "model").First(ctx, &out)
	if err != nil || out.Name != "model" {
		t.Fatalf("Model first = %+v, %v", out, err)
	}
}

func TestBuildFilterMergesMultipleOpsOnSameField(t *testing.T) {
	q := &Query{}
	q.Where("age", ">", 18).Where("age", "<", 30)
	filter := q.buildFilter()
	ageFilter, ok := filter["age"].(bson.M)
	if !ok {
		t.Fatalf("age filter is not bson.M: %#v", filter["age"])
	}
	if ageFilter["$gt"] != 18 || ageFilter["$lt"] != 30 {
		t.Fatalf("merged filter = %#v, want $gt:18 $lt:30", ageFilter)
	}
}

func TestBuildFilterWhereNull(t *testing.T) {
	q := &Query{}
	q.WhereNull("deleted_at").Where("active", true)
	filter := q.buildFilter()
	if filter["deleted_at"] != nil {
		t.Fatalf("WhereNull should set field to nil, got %#v", filter["deleted_at"])
	}
	if _, exists := filter["deleted_at"]; !exists {
		t.Fatal("WhereNull field should exist in filter with nil value")
	}
	if filter["active"] != true {
		t.Fatalf("other conditions lost: %#v", filter)
	}
}

func TestBuildFilterWhereBetween(t *testing.T) {
	q := &Query{}
	q.WhereBetween("age", 18, 30)
	filter := q.buildFilter()
	ageFilter, ok := filter["age"].(bson.M)
	if !ok {
		t.Fatalf("age filter is not bson.M: %#v", filter["age"])
	}
	if ageFilter["$gte"] != 18 || ageFilter["$lte"] != 30 {
		t.Fatalf("between filter = %#v", ageFilter)
	}
}

func TestPluralize(t *testing.T) {
	cases := map[string]string{
		"user":     "users",
		"city":     "cities",
		"category": "categories",
		"day":      "days",
		"key":      "keys",
		"boy":      "boys",
		"bus":      "buses",
		"address":  "addresses",
		"fox":      "foxes",
		"match":    "matches",
		"dish":     "dishes",
	}
	for input, want := range cases {
		if got := pluralize(input); got != want {
			t.Fatalf("pluralize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeUpdate(t *testing.T) {
	plain := normalizeUpdate(bson.M{"name": "Tom", "age": 20})
	if set, ok := plain.(bson.M)["$set"]; !ok {
		t.Fatalf("plain update not wrapped: %#v", plain)
	} else {
		inner := set.(bson.M)
		if inner["name"] != "Tom" {
			t.Fatalf("$set content = %#v", inner)
		}
	}

	withOp := normalizeUpdate(bson.M{"$inc": bson.M{"count": 1}})
	if _, ok := withOp.(bson.M)["$inc"]; !ok {
		t.Fatalf("operator update should pass through: %#v", withOp)
	}
}
