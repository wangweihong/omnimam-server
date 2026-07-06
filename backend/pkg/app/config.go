package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wangweihong/gotoolbox/pkg/homedir"

	"github.com/gosuri/uitable"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const configFlagName = "config"

var cfgFile string

// nolint: gochecknoinits
func init() {
	pflag.StringVarP(&cfgFile, configFlagName, "c", cfgFile, "Read configuration from specified `FILE`, "+
		"support JSON, TOML, YAML, HCL, or Java properties formats.")
}

// addConfigFlag adds flags for a specific server to the specified FlagSet
// object.
// 优先级:
// 1. 显式调用 viper.Set 设置的配置值
// 2. 命令行参数
// 3. 环境变量
// 4. 配置文件
// 5. key/value 存储.
func addConfigFlag(basename string, fs *pflag.FlagSet) {
	// 添加--config标志到标志集中
	fs.AddFlag(pflag.Lookup(configFlagName))

	viper.AutomaticEnv()
	viper.SetEnvPrefix(strings.Replace(strings.ToUpper(basename), "-", "_", -1))
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// 添加cobra初始化方法,该方法将在cobra执行标志解析后执行
	// 不在Initialize进行配置加载。在这里进行配置加载会影响到--version flag无法执行
	cobra.OnInitialize(func() {})
}

func printConfig() {
	if keys := viper.AllKeys(); len(keys) > 0 {
		fmt.Printf("%v Configuration items:\n", progressMessage)
		table := uitable.New()
		table.Separator = " "
		table.MaxColWidth = 80
		table.RightAlign(0)
		for _, k := range keys {
			table.AddRow(fmt.Sprintf("%s:", k), viper.Get(k))
		}
		fmt.Printf("%v", table)
	}
}

// loadConfig reads in config file and ENV variables if set.
func loadConfig(cfg string, defaultName string) error {
	if cfg != "" {
		viper.SetConfigFile(cfg)
	} else {
		viper.AddConfigPath(".")
		// 2. 添加~/<defaultName>以及/etc/defaultName作为配置路径
		if names := strings.Split(defaultName, "-"); len(names) > 1 {
			viper.AddConfigPath(filepath.Join(homedir.HomeDir(), "."+names[0]))
			viper.AddConfigPath(filepath.Join("/etc", names[0]))
		}
		// 设置配置名
		viper.SetConfigName(defaultName)
	}

	// Use config file from the flag.
	viper.SetConfigType("yaml") // set the type of the configuration to yaml.
	viper.AutomaticEnv()        // read in environment variables that match.
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// 加载配置文件
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read configuration file(%s): %w", cfgFile, err)
	}
	return nil
}
