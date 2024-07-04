package gen

type JetModelGenCmd struct {
	RootCmd RootParams `mapstructure:"root_cmd"`
}

type RootParams struct {
	// Database fields
	DbSchema     string   `mapstructure:"db_schema"`
	DbDriver     DBDriver `mapstructure:"db_driver"`
	DbUser       string   `mapstructure:"db_user"`
	DbPassword   string   `mapstructure:"db_password"`
	DbHost       string   `mapstructure:"db_host"`
	DbPort       int      `mapstructure:"db_port"`
	DbName       string   `mapstructure:"db_name"`
	DbSslMode    string   `mapstructure:"db_ssl_mode"`
	DbSslKey     string   `mapstructure:"db_ssl_mode"`
	DbSslRootCrt string   `mapstructure:"db_ssl_root_crt"`
	DbSslCrt     string   `mapstructure:"db_ssl_crt"`

	// Directory/file fields
	GoDir        string `mapstructure:"go_dir"`
	TsDir        string `mapstructure:"ts_dir"`
	TsFile       string `mapstructure:"ts_file"`
	RemoveGenDir bool   `mapstructure:"remove_gen_dir"`

	// Data type fields
	NewTimestampName string `mapstructure:"new_timestamp_name"`
	NewBigintName    string `mapstructure:"new_bigint_name"`
	NewUUIDName      string `mapstructure:"new_uuid_name"`

	NewTimestampPath string `mapstructure:"new_timestamp_path"`
	NewBigintPath    string `mapstructure:"new_bigint_path"`
	NewUUIDPath      string `mapstructure:"new_uuid_path"`

	ExcludedTableFieldTags []string `mapstructure:"excluded_table_field_tags"`
}
