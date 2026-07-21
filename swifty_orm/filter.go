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

import "go.mongodb.org/mongo-driver/bson"

type condition struct {
	field string
	op    string
	value interface{}
}

func parseWhere(args ...interface{}) condition {
	switch len(args) {
	case 2:
		return condition{field: args[0].(string), op: "=", value: args[1]}
	case 3:
		return condition{field: args[0].(string), op: args[1].(string), value: args[2]}
	default:
		return condition{}
	}
}

var opMap = map[string]string{
	"!=":   "$ne",
	"<>":   "$ne",
	">":    "$gt",
	">=":   "$gte",
	"<":    "$lt",
	"<=":   "$lte",
	"$in":  "$in",
	"$nin": "$nin",
}

func (q *Query) buildFilter() bson.M {
	filter := bson.M{}
	for _, c := range q.conditions {
		applyCondition(filter, c)
	}
	if len(q.orGroups) > 0 {
		orClauses := []bson.M{filter}
		for _, group := range q.orGroups {
			clause := bson.M{}
			for _, c := range group {
				applyCondition(clause, c)
			}
			orClauses = append(orClauses, clause)
		}
		return bson.M{"$or": orClauses}
	}
	return filter
}

func applyCondition(filter bson.M, c condition) {
	switch c.op {
	case "=":
		filter[c.field] = c.value
	case "null":
		filter[c.field] = nil
	case "notNull":
		mergeFieldOp(filter, c.field, "$ne", nil)
	case "between":
		pair := c.value.([2]interface{})
		mergeFieldOp(filter, c.field, "$gte", pair[0])
		mergeFieldOp(filter, c.field, "$lte", pair[1])
	default:
		mongoOp, ok := opMap[c.op]
		if !ok {
			mongoOp = c.op
		}
		mergeFieldOp(filter, c.field, mongoOp, c.value)
	}
}

func mergeFieldOp(filter bson.M, field string, op string, value interface{}) {
	existing, ok := filter[field]
	if ok {
		if m, isMap := existing.(bson.M); isMap {
			m[op] = value
			return
		}
	}
	filter[field] = bson.M{op: value}
}

func (q *Query) buildProjection() bson.M {
	if len(q.fields) == 0 {
		return nil
	}
	proj := bson.M{}
	for _, f := range q.fields {
		proj[f] = 1
	}
	return proj
}
