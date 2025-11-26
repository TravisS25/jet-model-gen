package gen

import (
	"bufio"
	"bytes"
	"database/sql"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"log"
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

type TagFormat string

type Tag struct {
	Name   string    `mapstructure:"name"`
	Format TagFormat `mapstructure:"format"`
}

type TsConvert struct {
	OldType string `mapstructure:"old_type"`
	NewType string `mapstructure:"new_type"`
}

type GoImport struct {
	Path    string `mapstructure:"path"`
	NewType string `mapstructure:"new_type"`
}

type GoConvert struct {
	OldType string   `mapstructure:"old_type"`
	Import  GoImport `mapstructure:"import"`
}

const (
	CamelCaseTagFormat TagFormat = "camelCase"
	SnakeCaseTagFormat TagFormat = "snakeCase"
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
	Driver                 DBDriver
	BaseJetDir             string
	Schema                 string
	ExcludedTableFieldTags map[string]struct{}
	Tags                   []Tag
	TypeConverts           []GoConvert
}

type TsModelParams struct {
	TypeConverts []TsConvert
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
		return errors.Wrapf(err, "%s: error trying to get schema data", PACKAGE_NAME)
	}

	//newTableMetaList := make([]metadata.Table, 0, len(schemaMetadata.TablesMetaData))
	tableMap := make(map[string][]foreignKey, len(schemaMetadata.TablesMetaData))

	for _, table := range schemaMetadata.TablesMetaData {
		rows, err := db.Query(getForeignKeyQuery(params.Driver, params.Schema, table.Name))
		if err != nil {
			return errors.Wrapf(err, "%s: error querying foreign tables", PACKAGE_NAME)
		}

		fks := make([]foreignKey, 0, 10)

		for rows.Next() {
			var columnName, foreignTableName string

			if err = rows.Scan(&columnName, &foreignTableName); err != nil {
				return errors.Wrapf(err, "%s: error trying to scan foreign row", PACKAGE_NAME)
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

								for _, convert := range params.TypeConverts {
									if convert.OldType == col.DataType.Name {
										field = field.UseType(template.Type{
											Name:       getFieldName(col, convert.Import.NewType),
											ImportPath: convert.Import.Path,
										})
									}
								}

								excludeJsonTag := false

								if params.ExcludedTableFieldTags != nil {
									excludedField := table.Name + "." + col.Name

									if _, ok := params.ExcludedTableFieldTags[excludedField]; ok {
										excludeJsonTag = true
									}
								}

								tags := make([]string, 0, len(params.Tags))

								for _, tag := range params.Tags {
									var colName string

									switch tag.Format {
									case SnakeCaseTagFormat:
										colName = snaker.CamelToSnake(col.Name)
									default:
										colName = snaker.ForceLowerCamelIdentifier(col.Name)
									}

									if tag.Name == "json" && excludeJsonTag {
										tags = append(tags, fmt.Sprintf(`%s:"%s"`, tag.Name, "-"))
										continue
									}

									tags = append(tags, fmt.Sprintf(`%s:"%s"`, tag.Name, colName))
								}

								field = field.UseTags(tags...)
								return field
							})
					}),
				)
		})

	log.Printf("Generating go files...")

	if err = template.ProcessSchema(params.BaseJetDir, schemaMetadata, tmpl); err != nil {
		return errors.Wrapf(err, "%s: error processing schema", PACKAGE_NAME)
	}

	log.Printf("Updating go files...")

	goModelDir := filepath.Join(params.BaseJetDir, params.Schema, "model")
	if err = filepath.WalkDir(goModelDir, func(path string, entry fs.DirEntry, err error) error {
		if !entry.IsDir() {
			if !strings.HasSuffix(entry.Name(), ".go") {
				return nil
			}

			origFile, err := os.Open(path)
			if err != nil {
				return errors.Wrapf(err, "%s: error opening file %q", PACKAGE_NAME, path)
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
							return errors.Wrapf(err, "%s: error trying to write to buffer", PACKAGE_NAME)
						}
					}
				}

				if _, err = bufWriter.WriteString(line + "\n"); err != nil {
					return errors.Wrapf(err, "%s: error trying to write to buffer", PACKAGE_NAME)
				}
			}

			// Check for scanning errors
			if err := scanner.Err(); err != nil {
				return errors.Wrapf(err, "%s: error scanning file", PACKAGE_NAME)
			}

			// Close the original file
			origFile.Close()

			formattedFile, err := format.Source(bufWriter.Bytes())
			if err != nil {
				return errors.Wrapf(err, "%s: error trying to format file", PACKAGE_NAME)
			}

			tempFilePath := "jet_model_gen_test.go"
			tempFile, err := os.Create(tempFilePath)
			if err != nil {
				return errors.Wrapf(err, "%s: error creating temporary file", PACKAGE_NAME)
			}

			defer tempFile.Close()

			if _, err = tempFile.Write(formattedFile); err != nil {
				return errors.Wrapf(err, "%s: error writing to file", PACKAGE_NAME)
			}

			// Remove the original file
			err = os.Remove(path)
			if err != nil {
				return errors.Wrapf(err, "%s: error removing original file", PACKAGE_NAME)
			}

			// Rename the temporary file to the original file name
			err = os.Rename(tempFilePath, path)
			if err != nil {
				return errors.Wrapf(err, "%s: error renaming temporary file", PACKAGE_NAME)
			}
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func GenerateTsModels(goModelDir, tsFile string, params TsModelParams) error {
	strConv := []string{
		"int64",
		"float64",
		"string",
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

	if err = os.MkdirAll(filepath.Dir(tsFile), os.ModePerm); err != nil {
		return errors.Wrapf(
			err,
			"%s: error creating directory %q",
			PACKAGE_NAME,
			filepath.Dir(tsFile),
		)
	}

	newFile, err := os.Create(
		filepath.Join(filepath.Dir(tsFile), filepath.Base(tsFile)),
	)
	if err != nil {
		return errors.Wrapf(
			err,
			"%s: error creating ts file %q",
			PACKAGE_NAME,
			tsFile,
		)
	}

	defer newFile.Close()

	log.Printf("Generating ts files...")

	return filepath.Walk(goModelDir, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			if !strings.HasSuffix(info.Name(), ".go") {
				return nil
			}

			openFile, err := os.Open(path)
			if err != nil {
				return errors.Wrapf(err, "%s: error trying to open file %q", PACKAGE_NAME, path)
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

					return errors.Wrapf(err, "%s: error trying to open file %q", PACKAGE_NAME, path)
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

					var fieldType, newFieldType, fieldName string

					if snaker.IsInitialism(lineArr[0]) {
						fieldName = strings.ToLower(lineArr[0])
					} else {
						fn := []rune(lineArr[0])
						fn[0] = unicode.ToLower(fn[0])
						fieldName = string(fn)
					}

					if lineArr[1][0] == '*' {
						fieldType = lineArr[1][1:]
					} else {
						fieldType = lineArr[1]
					}

					for _, v := range strConv {
						if fieldType == v {
							newFieldType = "string"
						}
					}

					for _, v := range numConv {
						if fieldType == v {
							newFieldType = "number"
						}
					}

					if fieldType == "bool" {
						newFieldType = "boolean"
					}

					if newFieldType == "" {
						for _, kv := range params.TypeConverts {
							if strings.Contains(lineArr[1], kv.OldType) {
								fieldType = kv.NewType
								break
							}
						}
					}

					if newFieldType == "" {
						newFieldType = fieldType
					}

					newLine := fmt.Sprintf("\t%s?: %s\n", fieldName, newFieldType)

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
