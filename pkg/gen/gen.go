package gen

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/TravisS25/jet-model-gen/pkg/queryset"
	"github.com/go-jet/jet/v2/generator/metadata"
	"github.com/go-jet/jet/v2/generator/template"
	postgres2 "github.com/go-jet/jet/v2/postgres"
	"github.com/pkg/errors"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/kenshaw/snaker"
)

const (
	PACKAGE_NAME         = "jet-model-gen"
	JET_PASSWORD_ENV_VAR = "JET_MODEL_GEN_DB_PASSWORD"
)

type foreignKey struct {
	ColumnName       string
	ForeignTableName string
}

type GoModelParams struct {
	Driver           DBDriver
	GoDir            string
	Schema           string
	NewTimestampName string
	NewBigintName    string
	NewUUIDName      string
	NewTimestampPath string
	NewBigintPath    string
	NewUUIDPath      string
}

func GenerateGoModels(db *sql.DB, params GoModelParams) error {
	var querySet metadata.DialectQuerySet

	switch params.Driver {
	case PostgresDriver:
		querySet = &queryset.PostgresQuerySet{}
	case MysqlDriver:
		querySet = &queryset.MySqlQuerySet{}
	default:
		querySet = &queryset.SqliteQuerySet{}
	}

	schemaMetadata, err := metadata.GetSchema(db, querySet, params.Schema)
	if err != nil {
		return errors.WithStack(
			fmt.Errorf(PACKAGE_NAME+": error trying to get schema data: %s", err),
		)
	}

	newTableMetaList := make([]metadata.Table, 0, len(schemaMetadata.TablesMetaData))

	for _, table := range schemaMetadata.TablesMetaData {
		rows, err := db.QueryContext(
			context.Background(),
			getForeignKeyQuery(params.Driver, params.Schema, table.Name),
		)

		if err != nil {
			return errors.WithStack(
				fmt.Errorf(PACKAGE_NAME+": error querying foreign tables: %s", err),
			)
		}

		fks := []foreignKey{}

		for rows.Next() {
			var columnName, foreignTableName string

			if err = rows.Scan(&columnName, &foreignTableName); err != nil {
				return errors.WithStack(
					fmt.Errorf(PACKAGE_NAME+": error trying to scan foreign row: %s", err),
				)
			}

			fks = append(fks, foreignKey{
				ColumnName:       columnName,
				ForeignTableName: foreignTableName,
			})
		}

		cols := make([]metadata.Column, 0, len(table.Columns)+len(fks))
		cols = append(cols, table.Columns...)

		for _, v := range fks {
			cols = append(cols, metadata.Column{
				Name: v.ColumnName[:len(v.ColumnName)-3],
				DataType: metadata.DataType{
					Name: v.ForeignTableName,
					Kind: metadata.UserDefinedType,
				},
			})
		}

		newTableMetaList = append(newTableMetaList, metadata.Table{
			Name:    table.Name,
			Columns: cols,
		})
	}

	schemaMetadata.TablesMetaData = newTableMetaList

	tmpl := template.Default(postgres2.Dialect).
		UseSchema(func(schema metadata.Schema) template.Schema {
			return template.DefaultSchema(schema).
				UseModel(template.DefaultModel().
					UseTable(func(table metadata.Table) template.TableModel {
						return template.DefaultTableModel(table).
							UseField(func(col metadata.Column) template.TableModelField {
								field := template.DefaultTableModelField(col)
								tags := []string{
									`json:"` + snaker.ForceLowerCamelIdentifier(col.Name) + `"`,
									`db:"` + col.Name + `"`,
								}

								if col.DataType.Kind == metadata.UserDefinedType {
									tags = append(tags, `alias:"`+col.Name+`"`)
									field = field.UseType(template.Type{
										Name: "*" + snaker.ForceCamelIdentifier(col.DataType.Name),
									})
								}

								if params.NewBigintName != "" && strings.Contains(col.DataType.Name, "bigint") {
									field = field.UseType(template.Type{
										Name:       params.NewBigintName,
										ImportPath: params.NewBigintPath,
									})
								}

								if params.NewTimestampName != "" && strings.Contains(col.DataType.Name, "timestamp") {
									field = field.UseType(template.Type{
										Name:       params.NewTimestampName,
										ImportPath: params.NewTimestampPath,
									})
								}

								if params.NewUUIDName != "" && strings.Contains(col.DataType.Name, "uuid") {
									field = field.UseType(template.Type{
										Name:       params.NewUUIDName,
										ImportPath: params.NewUUIDPath,
									})
								}

								field = field.UseTags(tags...)
								return field
							})
					}),
				)
		})

	fmt.Printf("Generating go files....\n")

	if err = template.ProcessSchema(params.GoDir, schemaMetadata, tmpl); err != nil {
		return fmt.Errorf("error processing schema: %s", err)
	}

	return nil
}

func GenerateTsModels(modelDir, tsDir, tsFile string) error {
	strConv := []string{
		"Int64",
		"int64",
		"float64",
		"string",
		"uuid",
		"time",
	}

	numConv := []string{
		"int8",
		"int16",
		"int",
		"int32",
		"float32",
	}

	space := regexp.MustCompile(`\s+`)

	var err error

	if err = os.MkdirAll(tsDir, os.ModePerm); err != nil {
		return errors.WithStack(err)
	}

	newFile, err := os.Create(filepath.Join(tsDir, tsFile))
	if err != nil {
		return errors.WithStack(err)
	}

	defer newFile.Close()

	fmt.Printf("Generating ts files....\n")

	return filepath.Walk(modelDir, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			if !strings.HasSuffix(info.Name(), ".go") {
				return nil
			}

			openFile, err := os.Open(path)
			if err != nil {
				return errors.WithStack(err)
			}

			defer openFile.Close()

			newFileWriter := bufio.NewWriter(newFile)
			openFileReader := bufio.NewReader(openFile)

			withinStruct := false

			for {
				l, err := openFileReader.ReadString('\n')

				if err != nil {
					if err == io.EOF {
						break
					}

					return errors.WithStack(err)
				}

				ajustedLine := strings.TrimSpace(space.ReplaceAllString(l, " "))

				if strings.Contains(ajustedLine, " struct {") {
					structArr := strings.Split(ajustedLine, " ")
					newFileWriter.WriteString(fmt.Sprintf("export interface %s {\n", structArr[1]))
					withinStruct = true
					continue
				}

				if strings.TrimSpace(ajustedLine) == "}" && withinStruct {
					newFileWriter.WriteString("}\n\n")
					break
				}

				if withinStruct {
					lineArr := strings.Split(ajustedLine, " ")

					var fieldType, fieldName string

					if snaker.IsInitialism(lineArr[0]) {
						fieldName = strings.ToLower(lineArr[0])
					} else {
						fn := []rune(lineArr[0])
						fn[0] = unicode.ToLower(fn[0])
						fieldName = string(fn)
					}

					if lineArr[1][0] == '*' {
						fieldType = lineArr[1][1:len(lineArr[1])]
					}

					for _, v := range strConv {
						if strings.Contains(lineArr[1], v) {
							fieldType = "string"
						}
					}

					for _, v := range numConv {
						if strings.Contains(lineArr[1], v) {
							fieldType = "number"
						}
					}

					if lineArr[1] == "bool" || lineArr[1] == "*bool" {
						fieldType = "boolean"
					}

					if fieldType == "" {
						fieldType = lineArr[1]
					}

					newLine := fmt.Sprintf("\t%s?: %s\n", fieldName, fieldType)

					if _, err = newFileWriter.WriteString(newLine); err != nil {
						return errors.WithStack(err)
					}
				}
			}

			if err = newFileWriter.Flush(); err != nil {
				return errors.WithStack(err)
			}
		}

		return nil
	})
}

func getForeignKeyQuery(driver DBDriver, schema, tableName string) string {
	switch driver {
	case PostgresDriver:
		return fmt.Sprintf(
			`
			select
				kcu.column_name,
				ccu.table_name AS "foreign_table_name"
			from
				information_schema.table_constraints AS tc
				JOIN information_schema.key_column_usage AS kcu
				ON tc.constraint_name = kcu.constraint_name
				and tc.table_schema = kcu.table_schema
				JOIN information_schema.constraint_column_usage AS ccu
				ON ccu.constraint_name = tc.constraint_name
				and ccu.table_schema = tc.table_schema
			where
				tc.table_schema = '%s'
			and
				tc.constraint_type = 'FOREIGN KEY'
			and
				tc.table_name='%s';
			`,
			schema,
			tableName,
		)
	case MysqlDriver:
		return fmt.Sprintf(
			`
			select
				column_name,
				referenced_table_name as "foreign_table_name"
			from
				information_schema.key_column_usgae
			where
				table_name = '%s'
			and
				referenced_table_name is not null;
			`,
			tableName,
		)
	default:
		return fmt.Sprintf(
			`
			select
				"from" as "column_name",
				"table" as "foreign_table_name"
			from
				pragma_foreign_key_list('%s');
			`,
			tableName,
		)
	}
}
