package schemamcp

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// TableColumn represents a column definition in a table.
type TableColumn struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	NotNull      bool   `json:"not_null"`
	DefaultValue string `json:"default_value,omitempty"`
	IsPrimaryKey bool   `json:"is_primary_key"`
}

// TableSchema represents a single table and its columns/indexes.
type TableSchema struct {
	Name    string        `json:"name"`
	Columns []TableColumn `json:"columns"`
	Indexes []string      `json:"indexes,omitempty"`
}

// DatabaseSchema wraps all extracted tables from a database.
type DatabaseSchema struct {
	Driver          string        `json:"driver"`
	Path            string        `json:"path"`
	Tables          []TableSchema `json:"tables"`
	Markdown        string        `json:"markdown"`
	EstimatedTokens int           `json:"estimated_tokens"`
}

// InspectSQLiteSchema opens a SQLite database and extracts its schema in pure Go.
func InspectSQLiteSchema(dbPath string) (*DatabaseSchema, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	defer db.Close()

	// Query all user tables
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name;`)
	if err != nil {
		return nil, fmt.Errorf("query tables: %w", err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tableNames = append(tableNames, name)
		}
	}

	var tables []TableSchema
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# SCHEMA DO BANCO: %s (%d tabelas)\n\n", dbPath, len(tableNames)))

	for _, tbl := range tableNames {
		colRows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s);", tbl))
		if err != nil {
			continue
		}

		var cols []TableColumn
		for colRows.Next() {
			var cid int
			var cname, ctype string
			var notnull, pk int
			var dflt *string

			if err := colRows.Scan(&cid, &cname, &ctype, &notnull, &dflt, &pk); err == nil {
				dfltStr := ""
				if dflt != nil {
					dfltStr = *dflt
				}
				cols = append(cols, TableColumn{
					Name:         cname,
					Type:         ctype,
					NotNull:      notnull == 1,
					DefaultValue: dfltStr,
					IsPrimaryKey: pk >= 1,
				})
			}
		}
		colRows.Close()

		// Get indexes
		idxRows, err := db.Query(fmt.Sprintf("PRAGMA index_list(%s);", tbl))
		var indexes []string
		if err == nil {
			for idxRows.Next() {
				var seq int
				var iname string
				var uniq int
				var origin string
				var partial int
				if err := idxRows.Scan(&seq, &iname, &uniq, &origin, &partial); err == nil {
					if !strings.HasPrefix(iname, "sqlite_") {
						indexes = append(indexes, iname)
					}
				}
			}
			idxRows.Close()
		}

		tables = append(tables, TableSchema{
			Name:    tbl,
			Columns: cols,
			Indexes: indexes,
		})

		// Format into lean Markdown
		sb.WriteString(fmt.Sprintf("### Tabela `%s`\n", tbl))
		sb.WriteString("| Coluna | Tipo | PK | Not Null | Padrão |\n")
		sb.WriteString("|---|---|:---:|:---:|---|\n")
		for _, c := range cols {
			pkStr := "-"
			if c.IsPrimaryKey {
				pkStr = "✅"
			}
			nnStr := "-"
			if c.NotNull {
				nnStr = "✅"
			}
			defVal := c.DefaultValue
			if defVal == "" {
				defVal = "-"
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s |\n", c.Name, c.Type, pkStr, nnStr, defVal))
		}
		if len(indexes) > 0 {
			sb.WriteString(fmt.Sprintf("*Índices:* `%s`\n", strings.Join(indexes, "`, `")))
		}
		sb.WriteString("\n")
	}

	markdown := strings.TrimSpace(sb.String())
	return &DatabaseSchema{
		Driver:          "sqlite",
		Path:            dbPath,
		Tables:          tables,
		Markdown:        markdown,
		EstimatedTokens: len(markdown) / 4,
	}, nil
}
