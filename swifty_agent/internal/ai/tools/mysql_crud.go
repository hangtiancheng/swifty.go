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

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// MysqlCrudInput defines the input for the MySQL CRUD tool.
type MysqlCrudInput struct {
	DSN         string `json:"dsn" jsonschema:"description=The Data Source Name for connecting to the MySQL database, including username, password, host, port, and database name"`
	SQL         string `json:"sql" jsonschema:"description=The SQL query to execute against the MySQL database"`
	OperateType string `json:"operate_type" jsonschema:"description=The type of SQL operation: query, insert, update, or delete"`
}

// NewMysqlCrudTool creates a tool that executes SQL queries against a MySQL database.
// The web version removes the interactive stdin confirmation present in the source
// project and executes the SQL directly. Query results are returned in JSON format.
// Construction errors (rare; only JSON-schema inference) are returned to the caller
// instead of terminating the process via log.Fatal.
func NewMysqlCrudTool() (tool.InvokableTool, error) {
	t, err := utils.InferOptionableTool(
		"mysql_crud",
		"Execute SQL queries against a MySQL database and return results in JSON format. Supports query, insert, update, and delete operations. Results are formatted as JSON for easy parsing.",
		func(ctx context.Context, input *MysqlCrudInput, opts ...tool.Option) (string, error) {
			return execMysqlSql(input)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("infer mysql_crud tool: %w", err)
	}
	return t, nil
}

// execMysqlSql opens a GORM connection and executes the SQL exactly once based on
// operate_type: "query" returns rows as JSON; insert/update/delete return a success
// message. The previous implementation ran db.Exec then db.Raw for queries (double
// execution) and blocked on stdin for confirmation — both are fixed here.
func execMysqlSql(input *MysqlCrudInput) (string, error) {
	db, err := gorm.Open(mysql.Open(input.DSN), &gorm.Config{})
	if err != nil {
		return "", fmt.Errorf("open mysql: %w", err)
	}

	if input.OperateType == "query" {
		var results []map[string]any
		if err := db.Raw(input.SQL).Scan(&results).Error; err != nil {
			return "", fmt.Errorf("query mysql: %w", err)
		}
		b, err := json.Marshal(results)
		if err != nil {
			return "", fmt.Errorf("marshal query result: %w", err)
		}
		return string(b), nil
	}

	// insert / update / delete
	if err := db.Exec(input.SQL).Error; err != nil {
		return "", fmt.Errorf("exec mysql: %w", err)
	}
	resp := map[string]any{
		"success": true,
		"message": fmt.Sprintf("Executed %s sql", input.OperateType),
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("marshal exec result: %w", err)
	}
	return string(b), nil
}
