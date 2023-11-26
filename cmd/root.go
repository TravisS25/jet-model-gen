/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/TravisS25/jet-model-gen/pkg/gen"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stretchr/objx"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

const (
	baseDSN = "%s://%s:%s@%s:%d/%s?sslmode=%s&sslrootcert=%s&sslkey=%s&sslcert=%s"
)

var rootFlagParams = rootFlagNames{
	// Database fields
	DbDriver: flagName{
		LongHand: "db-driver",
	},
	DbHost: flagName{
		LongHand: "db-host",
	},
	DbPort: flagName{
		LongHand: "db-port",
	},
	DbUser: flagName{
		LongHand: "db-user",
	},
	DbPassword: flagName{
		LongHand: "db-password",
	},
	DbSchema: flagName{
		LongHand: "db-schema",
	},

	// Data type fields
	ConvertTimestamp: flagName{
		LongHand: "convert-timestamp",
	},
	ConvertDate: flagName{
		LongHand: "convert-date",
	},
	ConvertBigint: flagName{
		LongHand: "convert-bigint",
	},
	ConvertUUID: flagName{
		LongHand: "convert-uuid",
	},
	LanguageType: flagName{
		LongHand: "language-type",
	},

	// Directory/file fields
	GoDir: flagName{
		LongHand: "go-dir",
	},
	TsDir: flagName{
		LongHand: "ts-dir",
	},
	TsFile: flagName{
		LongHand: "ts-file",
	},
}

var languageTypeMap = map[gen.LanguageType]struct{}{
	gen.GoLanguageType: struct{}{},
	gen.TsLanguageType: struct{}{},
}

var dbDriverMap = map[gen.DBDriver]struct{}{
	gen.PostgresDriver: struct{}{},
	gen.MysqlDriver:    struct{}{},
	gen.SqliteDriver:   struct{}{},
}

type rootParms struct {
	DbDriver   gen.DBDriver
	DbSchema   string
	DbUser     string
	DbPassword string
	DbHost     string
	DbPort     int
	DbName     string

	GoDir  string
	TsDir  string
	TsFile string
}

type rootFlagNames struct {
	// Database fields
	DbSchema     flagName
	DbDriver     flagName
	DbUser       flagName
	DbPassword   flagName
	DbHost       flagName
	DbPort       flagName
	DbName       flagName
	DbSslMode    flagName
	DbSslKey     flagName
	DbSslRootCrt flagName
	DbSslCrt     flagName

	// Directory/file fields
	GoDir  flagName
	TsDir  flagName
	TsFile flagName

	// Data type fields
	ConvertTimestamp flagName
	ConvertDate      flagName
	ConvertBigint    flagName
	ConvertUUID      flagName
	LanguageType     flagName
}

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "jet-model-gen",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },

	PreRunE: func(cmd *cobra.Command, args []string) error {
		dbDriver, _ := cmd.Flags().GetString(rootFlagParams.DbDriver.LongHand)
		dbSchema, _ := cmd.Flags().GetString(rootFlagParams.DbSchema.LongHand)
		dbUser, _ := cmd.Flags().GetString(rootFlagParams.DbUser.LongHand)
		dbPassword, _ := cmd.Flags().GetString(rootFlagParams.DbPassword.LongHand)
		dbHost, _ := cmd.Flags().GetString(rootFlagParams.DbHost.LongHand)
		dbPort, _ := cmd.Flags().GetInt(rootFlagParams.DbPort.LongHand)
		dbName, _ := cmd.Flags().GetString(rootFlagParams.DbName.LongHand)

		goDir, _ := cmd.Flags().GetString(rootFlagParams.GoDir.LongHand)
		tsDir, _ := cmd.Flags().GetString(rootFlagParams.TsDir.LongHand)
		tsFile, _ := cmd.Flags().GetString(rootFlagParams.TsFile.LongHand)

		return rootCmdPreRunValidation(rootParms{
			DbDriver:   gen.DBDriver(dbDriver),
			DbSchema:   dbSchema,
			DbUser:     dbUser,
			DbPassword: dbPassword,
			DbHost:     dbHost,
			DbPort:     dbPort,
			DbName:     dbName,

			GoDir:  goDir,
			TsDir:  tsDir,
			TsFile: tsFile,
		})
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		var dbPort int
		var dbDriver gen.DBDriver
		var dbHost, dbUser, dbPassword, dbName, dbSchema, dbSslMode, dbSslRootCrt, dbSslKey, dbSslCrt string
		var convertTimestamp, convertDate, convertBigint, convertUUID string
		var goDir, tsDir, tsFile string

		if err = viper.ReadInConfig(); err == nil {
			rootCmd := objx.New(viper.GetStringMap("root_cmd"))

			// Database fields
			dbSchema = rootCmd.Get("db_schema").Str()
			dbDriver = gen.DBDriver(rootCmd.Get("db_driver").Str())
			dbUser = rootCmd.Get("db_user").Str()
			dbPassword = rootCmd.Get("db_password").Str()
			dbHost = rootCmd.Get("db_host").Str()
			dbPort = rootCmd.Get("db_port").Int()
			dbName = rootCmd.Get("db_name").Str()
			dbSslMode = rootCmd.Get("db_ssl_mode").Str()
			dbSslRootCrt = rootCmd.Get("db_ssl_root_cert").Str()
			dbSslKey = rootCmd.Get("db_ssl_key").Str()
			dbSslCrt = rootCmd.Get("db_ssl_cert").Str()

			// Directory/file fields
			goDir = rootCmd.Get("go_output_dir").Str()
			tsDir = rootCmd.Get("ts_output_dir").Str()
			tsFile = rootCmd.Get("ts_output_file").Str()

			// Data type fields
			convertTimestamp = rootCmd.Get("convert_timestamp").Str()
			convertDate = rootCmd.Get("convert_date").Str()
			convertBigint = rootCmd.Get("convert_bigint").Str()
			convertUUID = rootCmd.Get("convert_uuid").Str()
		}

		dbDriverCmd, _ := cmd.Flags().GetString(rootFlagParams.DbDriver.LongHand)
		dbSchemaCmd, _ := cmd.Flags().GetString(rootFlagParams.DbSchema.LongHand)
		dbUserCmd, _ := cmd.Flags().GetString(rootFlagParams.DbUser.LongHand)
		dbPasswordCmd, _ := cmd.Flags().GetString(rootFlagParams.DbPassword.LongHand)
		dbHostCmd, _ := cmd.Flags().GetString(rootFlagParams.DbHost.LongHand)
		dbPortCmd, _ := cmd.Flags().GetInt(rootFlagParams.DbPort.LongHand)
		dbSslModeCmd, _ := cmd.Flags().GetString(rootFlagParams.DbSslMode.LongHand)
		dbSslCrtCmd, _ := cmd.Flags().GetString(rootFlagParams.DbSslCrt.LongHand)
		dbSslRootCrtCmd, _ := cmd.Flags().GetString(rootFlagParams.DbSslRootCrt.LongHand)
		dbSslKeyCmd, _ := cmd.Flags().GetString(rootFlagParams.DbSslKey.LongHand)

		convertTimestampCmd, _ := cmd.Flags().GetString(rootFlagParams.ConvertTimestamp.LongHand)
		convertDateCmd, _ := cmd.Flags().GetString(rootFlagParams.ConvertDate.LongHand)
		convertBigintCmd, _ := cmd.Flags().GetString(rootFlagParams.ConvertBigint.LongHand)
		convertUUIDCmd, _ := cmd.Flags().GetString(rootFlagParams.ConvertUUID.LongHand)

		goDirCmd, _ := cmd.Flags().GetString(rootFlagParams.GoDir.LongHand)
		tsDirCmd, _ := cmd.Flags().GetString(rootFlagParams.TsDir.LongHand)
		tsFileCmd, _ := cmd.Flags().GetString(rootFlagParams.TsFile.LongHand)

		if dbDriverCmd != "" {
			dbDriver = gen.DBDriver(dbDriverCmd)
		}
		if dbSchemaCmd != "" {
			dbSchema = dbSchemaCmd
		}
		if dbUserCmd != "" {
			dbUser = dbUserCmd
		}
		if dbPasswordCmd != "" {
			dbPassword = dbPasswordCmd
		}
		if dbHostCmd != "" {
			dbHost = dbHostCmd
		}
		if dbPortCmd != 0 {
			dbPort = dbPortCmd
		}
		if dbSslModeCmd != "" {
			dbSslMode = dbSslModeCmd
		}
		if dbSslCrtCmd != "" {
			dbSslCrt = dbSslCrtCmd
		}
		if dbSslRootCrtCmd != "" {
			dbSslRootCrt = dbSslRootCrtCmd
		}
		if dbSslKeyCmd != "" {
			dbSslKeyCmd = dbSslKeyCmd
		}

		if convertTimestampCmd != "" {
			convertTimestamp = convertTimestampCmd
		}
		if convertDateCmd != "" {
			convertDate = convertDateCmd
		}
		if convertBigintCmd != "" {
			convertBigint = convertBigintCmd
		}
		if convertUUIDCmd != "" {
			convertUUID = convertUUIDCmd
		}

		if goDirCmd != "" {
			goDir = goDirCmd
		}
		if tsDirCmd != "" {
			tsDir = tsDirCmd
		}
		if tsFileCmd != "" {
			tsFile = tsFileCmd
		}

		dbURL := fmt.Sprintf(
			baseDSN,
			dbDriver,
			dbUser,
			dbPassword,
			dbHost,
			dbPort,
			dbName,
			dbSslMode,
			dbSslRootCrt,
			dbSslKey,
			dbSslCrt,
		)

		db, err := sql.Open(string(dbDriver), dbURL)
		if err != nil {
			return fmt.Errorf("error trying to connect to database: %s", err)
		}

		if err = gen.GenerateGoModels(
			db,
			gen.GoModelParams{
				Driver: dbDriver,
				GoDir: ,
			},
		); err != nil {
			return errors.WithStack(err)
		}

		if tsDir != "" && tsFile != "" {
			if err = gen.GenerateTsModels(goDir, tsDir, tsFile); err != nil {
				return errors.WithStack(err)
			}
		}

		return nil
	},
}

func rootCmdPreRunValidation(params rootParms) error {
	var err error
	var ok bool
	var rootCmdMap map[string]interface{}

	if err = viper.ReadInConfig(); err == nil {
		rootCmdFromFile := viper.Get("root_cmd")

		if rootCmdFromFile == nil {
			return errors.WithStack(fmt.Errorf(gen.PACKAGE_NAME + ": root_cmd key in config file must be set"))
		}

		if rootCmdMap, ok = rootCmdFromFile.(map[string]interface{}); !ok {
			return errors.WithStack(fmt.Errorf(gen.PACKAGE_NAME + ": root_cmd key must be dictionary type"))
		}
	} else {
		rootCmdMap = make(map[string]interface{})
	}

	rootObjx := objx.New(rootCmdMap)

	dbDriver := gen.DBDriver(rootObjx.Get("db_driver").Str())
	dbSchema := rootObjx.Get("db_schema").Str()
	dbUser := rootObjx.Get("db_user").Str()
	dbPassword := rootObjx.Get("db_password").Str()
	dbHost := rootObjx.Get("db_host").Str()
	dbPort := rootObjx.Get("db_port").Int()
	dbName := rootObjx.Get("db_name").Str()

	goDir := rootObjx.Get("go_dir").Str()
	tsDir := rootObjx.Get("ts_dir").Str()
	tsFile := rootObjx.Get("ts_file").Str()

	if params.DbDriver != "" {
		dbDriver = params.DbDriver
	}
	if params.DbSchema != "" {
		dbSchema = params.DbSchema
	}
	if params.DbUser != "" {
		dbUser = params.DbUser
	}
	if params.DbPassword != "" {
		dbPassword = params.DbPassword
	}
	if params.DbHost != "" {
		dbHost = params.DbHost
	}
	if params.DbPort != 0 {
		dbPort = params.DbPort
	}
	if params.DbName != "" {
		dbName = params.DbName
	}

	if dbDriver == "" {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME, ": --db-driver flag required if config file is not used"),
		)
	}

	if _, ok = dbDriverMap[dbDriver]; !ok {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME + ": must choose valid --db-driver.  Options are 'postgres', 'mysql', 'sqlite'"),
		)
	}

	if dbDriver == gen.PostgresDriver && dbSchema == "" {
		return errors.WithStack(fmt.Errorf(gen.PACKAGE_NAME + ": --schema flag must be set when --db-driver is set to 'postgres'"))
	}
	if dbUser == "" {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME, ": --db-user flag required if config file is not used"),
		)
	}
	if dbPassword == "" {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME, ": --db-password flag required if config file is not used"),
		)
	}
	if dbHost == "" {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME, ": --db-host flag required if config file is not used"),
		)
	}
	if dbPort == 0 {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME, ": --db-port flag required if config file is not used"),
		)
	}
	if dbName == "" {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME, ": --db-name flag required if config file is not used"),
		)
	}

	if goDir == "" && tsDir == "" && tsFile == "" {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME, ": must pass --go-dir or --ts-dir and --ts-file"),
		)
	}

	if (tsDir != "" && tsFile == "") || (tsDir == "" && tsFile != "") {
		return errors.WithStack(fmt.Errorf(gen.PACKAGE_NAME + ": --ts-dir and --ts-file must be set together"))
	}

	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.model_gen.yaml)")
	rootCmd.PersistentFlags().String(
		rootFlagParams.DbDriver.LongHand,
		"",
		"Database driver to connect to database.  Options are postgres, mysql, sqlite",
	)
	rootCmd.PersistentFlags().String(
		rootFlagParams.DbSchema.LongHand,
		"",
		"Schema to base model generation off.  Required if driver is 'postgres'",
	)
	rootCmd.PersistentFlags().String(
		rootFlagParams.DbUser.LongHand,
		"",
		"User used to connect to database",
	)
	rootCmd.PersistentFlags().String(
		rootFlagParams.DbPassword.LongHand,
		"",
		`Password for database user.  Can be set with '`+gen.DB_PASSWORD_ENV_VAR+`' env var`,
	)
	rootCmd.PersistentFlags().String(
		rootFlagParams.DbHost.LongHand,
		"",
		"Host of database to connect to",
	)
	rootCmd.PersistentFlags().String(
		rootFlagParams.DbPort.LongHand,
		"",
		"Port on host to connect to database",
	)
	rootCmd.PersistentFlags().String(
		rootFlagParams.DbName.LongHand,
		"",
		"Name of database to connect to",
	)
	rootCmd.PersistentFlags().String(
		rootFlagParams.DbSslMode.LongHand,
		"disable",
		"SSL mode of connection to database",
	)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".modelgen" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".db_model_gen")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
