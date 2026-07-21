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
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Query struct {
	collection *mongo.Collection
	engine     *Engine
	conditions []condition
	orGroups   [][]condition
	sort       bson.D
	limit      int64
	skip       int64
	fields     []string
}

func (q *Query) Where(args ...interface{}) *Query {
	q.conditions = append(q.conditions, parseWhere(args...))
	return q
}

func (q *Query) WhereIn(field string, values interface{}) *Query {
	q.conditions = append(q.conditions, condition{field: field, op: "$in", value: values})
	return q
}

func (q *Query) WhereNotIn(field string, values interface{}) *Query {
	q.conditions = append(q.conditions, condition{field: field, op: "$nin", value: values})
	return q
}

func (q *Query) WhereNull(field string) *Query {
	q.conditions = append(q.conditions, condition{field: field, op: "null"})
	return q
}

func (q *Query) WhereNotNull(field string) *Query {
	q.conditions = append(q.conditions, condition{field: field, op: "notNull"})
	return q
}

func (q *Query) WhereBetween(field string, low interface{}, high interface{}) *Query {
	q.conditions = append(q.conditions, condition{field: field, op: "between", value: [2]interface{}{low, high}})
	return q
}

func (q *Query) OrWhere(args ...interface{}) *Query {
	q.orGroups = append(q.orGroups, []condition{parseWhere(args...)})
	return q
}

func (q *Query) OrderBy(field string, direction ...string) *Query {
	dir := 1
	if len(direction) > 0 && (direction[0] == "desc" || direction[0] == "DESC") {
		dir = -1
	}
	q.sort = append(q.sort, bson.E{Key: field, Value: dir})
	return q
}

func (q *Query) Limit(n int64) *Query {
	q.limit = n
	return q
}

func (q *Query) Offset(n int64) *Query {
	q.skip = n
	return q
}

func (q *Query) Select(fields ...string) *Query {
	q.fields = fields
	return q
}
