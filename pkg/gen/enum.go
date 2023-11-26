package gen

type DBDriver string
type LanguageType string

var (
	PostgresDriver DBDriver = "postgres"
	SqliteDriver   DBDriver = "sqlite"
	MysqlDriver    DBDriver = "mysql"
)

var (
	GoLanguageType LanguageType = "go"
	TsLanguageType LanguageType = "ts"
)
