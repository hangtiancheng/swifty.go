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
