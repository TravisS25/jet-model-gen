package gen

import (
	"bufio"
	"bytes"
	"database/sql"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/TravisS25/jet-model-gen/internal/queryset"
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

	//newTableMetaList := make([]metadata.Table, 0, len(schemaMetadata.TablesMetaData))
	tableMap := make(map[string][]foreignKey, len(schemaMetadata.TablesMetaData))

	for _, table := range schemaMetadata.TablesMetaData {
		rows, err := db.Query(getForeignKeyQuery(params.Driver, params.Schema, table.Name))
		if err != nil {
			return errors.WithStack(
				fmt.Errorf(PACKAGE_NAME+": error querying foreign tables: %s", err),
			)
		}

		fks := make([]foreignKey, 0, 10)

		for rows.Next() {
			var columnName, foreignTableName string

			if err = rows.Scan(&columnName, &foreignTableName); err != nil {
				return errors.WithStack(
					fmt.Errorf(PACKAGE_NAME+": error trying to scan foreign row: %s", err),
				)
			}

			fks = append(fks, foreignKey{
				ColumnName:       columnName[:len(columnName)-3],
				ForeignTableName: foreignTableName,
			})
		}

		tableMap[table.Name] = fks
	}

	getFieldName := func(col metadata.Column, newName string) string {
		var fieldName string

		if col.IsNullable {
			fieldName = "*" + newName
		} else {
			fieldName = newName
		}

		return fieldName
	}

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
									`mapstructure:"` + col.Name + `"`,
									`alias:"` + col.Name + `"`,
								}

								if params.NewBigintName != "" && strings.Contains(col.DataType.Name, "bigint") {
									field = field.UseType(template.Type{
										Name:       getFieldName(col, params.NewBigintName),
										ImportPath: params.NewBigintPath,
									})
								}

								if params.NewTimestampName != "" && strings.Contains(col.DataType.Name, "timestamp") {
									field = field.UseType(template.Type{
										Name:       getFieldName(col, params.NewTimestampName),
										ImportPath: params.NewTimestampPath,
									})
								}

								if params.NewUUIDName != "" && strings.Contains(col.DataType.Name, "uuid") {
									field = field.UseType(template.Type{
										Name:       getFieldName(col, params.NewUUIDName),
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

	fmt.Printf("Updating go files....\n")

	goFileDir := filepath.Join(params.GoDir, params.Schema, "model")
	if err = filepath.WalkDir(goFileDir, func(path string, entry fs.DirEntry, err error) error {
		if !entry.IsDir() {
			if !strings.HasSuffix(entry.Name(), ".go") {
				return nil
			}

			origFile, err := os.Open(path)
			if err != nil {
				return errors.WithStack(err)
			}

			// Create a scanner to read lines
			scanner := bufio.NewScanner(origFile)
			tableName := strings.Split(entry.Name(), ".")[0]

			fileInfo, err := entry.Info()
			if err != nil {
				return err
			}

			buf := make([]byte, 0, fileInfo.Size()+512)
			bufWriter := bytes.NewBuffer(buf)

			// Read lines into the buffer
			for scanner.Scan() {
				line := scanner.Text()

				if strings.TrimSpace(line) == "}" {
					fks := tableMap[tableName]

					for _, fk := range fks {
						_, err = bufWriter.WriteString(
							snaker.SnakeToCamelIdentifier(fk.ColumnName) + " *" +
								snaker.SnakeToCamelIdentifier(fk.ForeignTableName) + "`" +
								fmt.Sprintf(
									`json:"%s" db:"%s" mapstructure:"%s" alias:"%s:%s"`,
									snaker.ForceLowerCamelIdentifier(fk.ColumnName),
									fk.ColumnName,
									fk.ColumnName,
									tableName,
									fk.ColumnName,
								) + "` \n",
						)
						if err != nil {
							return fmt.Errorf("error trying to write to buffer: %s", err)
						}
					}
				}

				if _, err = bufWriter.WriteString(line + "\n"); err != nil {
					return fmt.Errorf("error trying to write to buffer: %s", err)
				}
			}

			// Check for scanning errors
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("error scanning file: %s", err)
			}

			// Close the original file
			origFile.Close()

			formattedFile, err := format.Source(bufWriter.Bytes())
			if err != nil {
				return fmt.Errorf("error trying to format file: %s", err)
			}

			tempFilePath := "jet_model_gen_test.go"
			tempFile, err := os.Create(tempFilePath)
			if err != nil {
				return fmt.Errorf("error creating temporary file: %s", err)
			}

			defer tempFile.Close()

			if _, err = tempFile.Write(formattedFile); err != nil {
				return fmt.Errorf("error writing to file: %s", err)
			}

			// Remove the original file
			err = os.Remove(path)
			if err != nil {
				return fmt.Errorf("error removing original file: %s", err)
			}

			// Rename the temporary file to the original file name
			err = os.Rename(tempFilePath, path)
			if err != nil {
				return fmt.Errorf("error renaming temporary file: %s", err)
			}

		}

		return nil
	}); err != nil {
		return err
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
