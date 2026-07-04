package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

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
// For safety, it prompts for user confirmation before executing any SQL statement.
// Query results are returned in JSON format.
func NewMysqlCrudTool() tool.InvokableTool {
	t, err := utils.InferOptionableTool(
		"mysql_crud",
		"Execute SQL queries against a MySQL database and return results in JSON format. Supports query, insert, update, and delete operations. Results are formatted as JSON for easy parsing.",
		func(ctx context.Context, input *MysqlCrudInput, opts ...tool.Option) (string, error) {
			db, err := gorm.Open(mysql.Open(input.DSN), &gorm.Config{})
			if err != nil {
				log.Fatal(err)
			}

			// Prompt for user confirmation before executing SQL.
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Print("\nConfirm SQL execution (y/n): ", input.SQL)
			scanner.Scan()
			fmt.Println()
			if scanner.Text() != "y" {
				return "User cancelled SQL execution", nil
			}

			if err := db.Exec(input.SQL).Error; err != nil {
				log.Fatal(err)
			}

			if input.OperateType == "query" {
				var results []interface{}
				if err := db.Raw(input.SQL).Scan(&results).Error; err != nil {
					log.Fatal(err)
				}
				b, err := json.Marshal(results)
				return string(b), err
			}
			return "", nil
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	return t
}
