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
	"path/filepath"
	"strings"

	"github.com/TravisS25/jet-model-gen/pkg/gen"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stretchr/objx"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var rootFlagParams = rootFlagNames{
	// Database fields
	DbSchema: flagName{
		LongHand: "db-schema",
	},
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
	DbName: flagName{
		LongHand: "db-name",
	},
	DbSslMode: flagName{
		LongHand: "db-ssl-mode",
	},
	DbSslKey: flagName{
		LongHand: "db-ssl-key",
	},
	DbSslCrt: flagName{
		LongHand: "db-ssl-crt",
	},
	DbSslRootCrt: flagName{
		LongHand: "db-ssl-root-crt",
	},

	// Directory/file fields
	BaseJetDir: flagName{
		LongHand: "base-jet-dir",
	},
	TsFile: flagName{
		LongHand: "ts-file",
	},
	RemoveGenDir: flagName{
		LongHand: "remove-gen-dir",
	},
	Tags: flagName{
		LongHand: "tag",
	},
	GoConverts: flagName{
		LongHand: "go-convert",
	},
	TsConverts: flagName{
		LongHand: "ts-convert",
	},
	ExcludedTableFieldTags: flagName{
		LongHand: "excluded-table-field-tags",
	},
}

var dbDriverMap = map[gen.DBDriver]struct{}{
	gen.PostgresDriver: {},
	gen.MysqlDriver:    {},
	gen.SqliteDriver:   {},
}

type rootValidationParms struct {
	DbDriver   gen.DBDriver
	DbSchema   string
	DbUser     string
	DbPassword string
	DbHost     string
	DbPort     int
	DbName     string

	GoDir  string
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
	BaseJetDir             flagName
	TsFile                 flagName
	RemoveGenDir           flagName
	Tags                   flagName
	GoConverts             flagName
	TsConverts             flagName
	ExcludedTableFieldTags flagName
}

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "jet-model-gen",
	Short: "Generates models for different languages based on database",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		dbDriver, _ := cmd.Flags().GetString(rootFlagParams.DbDriver.LongHand)
		dbSchema, _ := cmd.Flags().GetString(rootFlagParams.DbSchema.LongHand)
		dbUser, _ := cmd.Flags().GetString(rootFlagParams.DbUser.LongHand)
		dbPassword, _ := cmd.Flags().GetString(rootFlagParams.DbPassword.LongHand)
		dbHost, _ := cmd.Flags().GetString(rootFlagParams.DbHost.LongHand)
		dbPort, _ := cmd.Flags().GetInt(rootFlagParams.DbPort.LongHand)
		dbName, _ := cmd.Flags().GetString(rootFlagParams.DbName.LongHand)

		baseJetDir, _ := cmd.Flags().GetString(rootFlagParams.BaseJetDir.LongHand)
		tsFile, _ := cmd.Flags().GetString(rootFlagParams.TsFile.LongHand)

		return rootCmdPreRunValidation(rootValidationParms{
			DbDriver:   gen.DBDriver(dbDriver),
			DbSchema:   dbSchema,
			DbUser:     dbUser,
			DbPassword: dbPassword,
			DbHost:     dbHost,
			DbPort:     dbPort,
			DbName:     dbName,

			GoDir:  baseJetDir,
			TsFile: tsFile,
		})
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		var dbPort int
		var removeGenDir bool
		var dbDriver gen.DBDriver
		var dbHost,
			dbUser,
			dbPassword,
			dbName,
			dbSchema,
			dbSslMode,
			dbSslRootCrt,
			dbSslKey,
			dbSslCrt string
		var excludedTableFieldTagMap map[string]struct{}
		var excludedTableFieldTags []string
		var baseJetDir, tsFile string
		var tagObjSlice, tsConvertObjSlice, goConvertObjSlice []objx.Map

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
			baseJetDir = rootCmd.Get("base_jet_dir").Str()
			tsFile = rootCmd.Get("ts_file").Str()
			removeGenDir = rootCmd.Get("remove_gen_dir").Bool()

			excludedTableFieldTags = rootCmd.Get("excluded_table_field_tags").
				StringSlice()
			tagObjSlice = rootCmd.Get("tag").ObjxMapSlice()
			tsConvertObjSlice = rootCmd.Get("ts_convert").ObjxMapSlice()
			goConvertObjSlice = rootCmd.Get("go_convert").ObjxMapSlice()
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

		baseJetDirCmd, _ := cmd.Flags().GetString(rootFlagParams.BaseJetDir.LongHand)
		tsFileCmd, _ := cmd.Flags().GetString(rootFlagParams.TsFile.LongHand)
		removeGenDirCmd, _ := cmd.Flags().GetBool(rootFlagParams.RemoveGenDir.LongHand)
		excludedTableFieldTagsCmd, _ := cmd.Flags().GetStringSlice(
			rootFlagParams.ExcludedTableFieldTags.LongHand,
		)
		tagsCmd, _ := cmd.Flags().GetStringSlice(rootFlagParams.Tags.LongHand)
		tsConvertsCmd, _ := cmd.Flags().GetStringSlice(rootFlagParams.TsConverts.LongHand)
		goConvertsCmd, _ := cmd.Flags().GetStringSlice(rootFlagParams.GoConverts.LongHand)

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
			dbSslKey = dbSslKeyCmd
		}
		if baseJetDirCmd != "" {
			baseJetDir = baseJetDirCmd
		}
		if tsFileCmd != "" {
			tsFile = tsFileCmd
		}
		if removeGenDirCmd {
			removeGenDir = true
		}

		if len(excludedTableFieldTagsCmd) > 0 {
			excludedTableFieldTags = excludedTableFieldTagsCmd
		}

		excludedTableFieldTagMap = make(map[string]struct{}, len(excludedTableFieldTags))
		for _, v := range excludedTableFieldTags {
			excludedTableFieldTagMap[v] = struct{}{}
		}

		var tags []gen.Tag

		if len(tagsCmd) > 0 {
			tags = make([]gen.Tag, 0, len(tagsCmd))

			for _, tag := range tagsCmd {
				tagArr := strings.Split(tag, ":")

				if len(tagArr) != 2 {
					return fmt.Errorf(
						"--tag passed are invalid format; should be %q",
						"<tag_name>:<'camelCase' | 'snakeCase'>",
					)
				}

				if tagArr[1] != string(gen.CamelCaseTagFormat) &&
					tagArr[1] != string(gen.SnakeCaseTagFormat) {
					return fmt.Errorf(
						"--tag[1] must be in format %q or %q",
						"camelCase",
						"snakeCase",
					)
				}

				tags = append(tags, gen.Tag{
					Name:   tagArr[0],
					Format: gen.TagFormat(tagArr[1]),
				})
			}
		} else {
			tags = make([]gen.Tag, 0, len(tagObjSlice))

			for _, tag := range tagObjSlice {
				tags = append(tags, gen.Tag{
					Name:   tag.Get("name").Str(),
					Format: gen.TagFormat(tag.Get("format").Str()),
				})
			}
		}

		var tsConverts []gen.TsConvert

		if len(tsConvertsCmd) > 0 {
			tsConverts = make([]gen.TsConvert, 0, len(tsConvertsCmd))

			for _, convert := range tsConvertsCmd {
				convertArr := strings.Split(convert, ":")

				if len(convertArr) != 2 {
					return fmt.Errorf(
						"--tags passed are invalid format; should be %q",
						"<tag_name>:<'camelCase' | 'snakeCase'>",
					)
				}
			}
		} else {
			tsConverts = make([]gen.TsConvert, 0, len(tsConvertObjSlice))

			for _, convert := range tsConvertObjSlice {
				tsConverts = append(tsConverts, gen.TsConvert{
					OldType: convert.Get("old_type").Str(),
					NewType: convert.Get("new_type").Str(),
				})
			}
		}

		var goConverts []gen.GoConvert

		if len(tsConvertsCmd) > 0 {
			goConverts = make([]gen.GoConvert, 0, len(goConvertsCmd))

			for _, convert := range tsConvertsCmd {
				convertArr := strings.Split(convert, ":")

				if len(convertArr) != 2 {
					return fmt.Errorf(
						"--tags passed are invalid format; should be %q",
						"<tag_name>:<'camelCase' | 'snakeCase'>",
					)
				}
			}
		} else {
			goConverts = make([]gen.GoConvert, 0, len(goConvertObjSlice))

			for _, convert := range goConvertObjSlice {
				goConverts = append(goConverts, gen.GoConvert{
					OldType: convert.Get("old_type").Str(),
					Import: gen.GoImport{
						NewType: convert.Get("import.new_type").Str(),
						Path:    convert.Get("import.path").Str(),
					},
				})
			}
		}

		dbURL := fmt.Sprintf(
			"%s://%s:%s@%s:%d/%s?sslmode=%s&sslrootcert=%s&sslkey=%s&sslcert=%s",
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
				Schema:                 dbSchema,
				Driver:                 dbDriver,
				BaseJetDir:             baseJetDir,
				ExcludedTableFieldTags: excludedTableFieldTagMap,
				Tags:                   tags,
			},
		); err != nil {
			return errors.WithStack(err)
		}

		if tsFile != "" {
			if err = gen.GenerateTsModels(
				filepath.Join(baseJetDir, dbSchema, "model"),
				tsFile,
				gen.TsModelParams{},
			); err != nil {
				return errors.WithStack(err)
			}
		}

		homeDir, _ := homedir.Dir()

		if removeGenDir && baseJetDir != homeDir {
			if err = os.RemoveAll(baseJetDir); err != nil {
				return errors.WithStack(err)
			}
		}

		return nil
	},
}

func rootCmdPreRunValidation(params rootValidationParms) error {
	var err error
	var ok bool
	var rootCmdMap map[string]interface{}

	if err = viper.ReadInConfig(); err == nil {
		rootCmdMap = viper.GetStringMap("root_cmd")

		if len(rootCmdMap) == 0 {
			return errors.WithStack(fmt.Errorf(gen.PACKAGE_NAME + ": 'root_cmd' key in config file must be set"))
		}
	} else {
		rootCmdMap = make(map[string]interface{})
	}

	rootObjx := objx.New(rootCmdMap)

	dbDriver := gen.DBDriver(rootObjx.Get("db_driver").Str())
	dbSchema := rootObjx.Get("db_schema").Str()
	dbUser := rootObjx.Get("db_user").Str()
	dbHost := rootObjx.Get("db_host").Str()
	dbPort := rootObjx.Get("db_port").Int()
	dbName := rootObjx.Get("db_name").Str()

	baseJetDir := rootObjx.Get("base_jet_dir").Str()
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
			fmt.Errorf(gen.PACKAGE_NAME + ": --db-driver flag required if config file is not used"),
		)
	}

	if _, ok = dbDriverMap[dbDriver]; !ok {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME + ": must choose valid --db-driver.  Options are 'postgres', 'mysql', 'sqlite'"),
		)
	}

	if dbDriver == gen.PostgresDriver && dbSchema == "" {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME + ": --schema flag must be set when --db-driver is set to 'postgres'"),
		)
	}
	if dbUser == "" {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME + ": --db-user flag required if config file is not used"),
		)
	}
	if dbHost == "" {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME + ": --db-host flag required if config file is not used"),
		)
	}
	if dbPort == 0 {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME + ": --db-port flag required if config file is not used"),
		)
	}
	if dbName == "" {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME + ": --db-name flag required if config file is not used"),
		)
	}

	if baseJetDir == "" && tsDir == "" && tsFile == "" {
		return errors.WithStack(
			fmt.Errorf(gen.PACKAGE_NAME + ": must pass --base-jet-dir or --ts-dir and --ts-file"),
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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.jet_model_gen.yaml)")

	////////////////////////////////
	// DATABASE FLAGS
	////////////////////////////////

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
		`Password for database user.  Can be set with '`+gen.JET_PASSWORD_ENV_VAR+`' env var`,
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
	rootCmd.PersistentFlags().String(
		rootFlagParams.DbSslKey.LongHand,
		"",
		"Private key if using ssl",
	)
	rootCmd.PersistentFlags().String(
		rootFlagParams.DbSslCrt.LongHand,
		"",
		"Public cert if using ssl",
	)
	rootCmd.PersistentFlags().String(
		rootFlagParams.DbSslRootCrt.LongHand,
		"",
		"Root cert if using ssl",
	)

	rootCmd.PersistentFlags().StringSlice(
		rootFlagParams.ExcludedTableFieldTags.LongHand,
		nil,
		"Determines what table field tags will be excluded from being exported.  Format should be <table_name>.<field_name>",
	)

	////////////////////////////////
	// DIRECTORY/FILE FLAGS
	////////////////////////////////

	rootCmd.PersistentFlags().String(
		rootFlagParams.BaseJetDir.LongHand,
		"",
		"Determines the base directory of where jet models and table will be generated",
	)
	rootCmd.PersistentFlags().String(
		rootFlagParams.TsFile.LongHand,
		"",
		"Determines what file to store generated ts models.  Should be absolute path",
	)
	rootCmd.PersistentFlags().Bool(
		rootFlagParams.RemoveGenDir.LongHand,
		false,
		"Determines whether to delete the generated go files",
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
		viper.SetConfigName(".jet_model_gen")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
