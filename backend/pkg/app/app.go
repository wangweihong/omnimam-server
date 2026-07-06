package app

import (
	"fmt"
	"os"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/spf13/pflag"

	//"github.com/wangweihong/gotoolbox/pkg/errors".

	"github.com/wangweihong/gotoolbox/pkg/terminal"
	"github.com/wangweihong/gotoolbox/pkg/version/verflag"

	"github.com/wangweihong/omnimam/backend/pkg/cli/globalflag"

	"github.com/wangweihong/gotoolbox/pkg/version"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wangweihong/gotoolbox/pkg/log"

	cliflag "github.com/wangweihong/omnimam/backend/pkg/cli/flag"
)

var (
	progressMessage = color.GreenString("==>")

	usageTemplate = fmt.Sprintf(`%s{{if .Runnable}}
  %s{{end}}{{if .HasAvailableSubCommands}}
  %s{{end}}{{if gt (len .Aliases) 0}}

%s
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

%s
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

%s{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  %s {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

%s
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

%s
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

%s{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "%s --help" for more information about a command.{{end}}
`,
		color.CyanString("Usage:"),
		color.GreenString("{{.UseLine}}"),
		color.GreenString("{{.CommandPath}} [command]"),
		color.CyanString("Aliases:"),
		color.CyanString("Examples:"),
		color.CyanString("Available Commands:"),
		color.GreenString("{{rpad .Name .NamePadding }}"),
		color.CyanString("Flags:"),
		color.CyanString("Global Flags:"),
		color.CyanString("Additional help topics:"),
		color.GreenString("{{.CommandPath}} [command]"),
	)
)

// App is the main structure of a cli application.
// It is recommended that an app be created with the app.NewApp() function.
type App struct {
	basename    string
	name        string
	description string
	options     CliOptions
	runFunc     RunFunc // 应用的运行入口
	silence     bool
	noVersion   bool
	noConfig    bool
	noPrintFlag bool
	commands    []*Command
	// cobra.Command schema: APPNAME COMMAND ARG --FLAG. For example: git clone URL --bare.
	// git: APPNAME, clone: COMMAND, URL: ARG, bare: FLAG
	args cobra.PositionalArgs
	cmd  *cobra.Command
}

// Option defines optional parameters for initializing the application
// structure.
type Option func(*App)

// WithOptions to open the application's function to read from the command line
// or read parameters from the configuration file.
func WithOptions(opt CliOptions) Option {
	return func(a *App) {
		a.options = opt
	}
}

// RunFunc defines the application's startup callback function.
type RunFunc func(basename string) error

// WithRunFunc is used to set the application startup callback function option.
func WithRunFunc(run RunFunc) Option {
	return func(a *App) {
		a.runFunc = run
	}
}

// WithDescription is used to set the description of the application.
func WithDescription(desc string) Option {
	return func(a *App) {
		a.description = desc
	}
}

// WithSilence sets the application to silent mode, in which the program startup
// information, configuration information, and version information are not
// printed in the console.
func WithSilence() Option {
	return func(a *App) {
		a.silence = true
	}
}

// WithNoVersion set the application does not provide version flag.
func WithNoVersion() Option {
	return func(a *App) {
		a.noVersion = true
	}
}

// WithNoConfig set the application does not provide config flag.
func WithNoConfig() Option {
	return func(a *App) {
		a.noConfig = true
	}
}

// WithNoPrintFlag set the application does not print config flag.
func WithNoPrintFlag() Option {
	return func(a *App) {
		a.noPrintFlag = true
	}
}

// WithValidArgs set the validation function to valid non-flag arguments.
// 自定义如何进行检测参数.
func WithValidArgs(args cobra.PositionalArgs) Option {
	return func(a *App) {
		a.args = args
	}
}

// WithDefaultValidArgs set default validation function to valid non-flag arguments.
func WithDefaultValidArgs() Option {
	return func(a *App) {
		a.args = func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}

			return nil
		}
	}
}

// NewApp creates a new application instance based on the given application name,
// binary name, and other options.
func NewApp(name string, basename string, opts ...Option) *App {
	a := &App{
		name:     name,
		basename: basename,
	}

	for _, o := range opts {
		o(a)
	}

	a.buildCommand()

	return a
}

func (a *App) buildCommand() {
	cmd := cobra.Command{
		Use:   FormatBaseName(a.basename),
		Short: a.name,
		Long:  a.description,
		// stop printing usage when the command errors
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          a.args,
	}
	cmd.SetUsageTemplate(usageTemplate)
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	cmd.Flags().SortFlags = true
	// 初始化标志位
	cliflag.InitFlags(cmd.Flags())

	if len(a.commands) > 0 {
		for _, command := range a.commands {
			cmd.AddCommand(command.cobraCommand())
		}
		cmd.SetHelpCommand(helpCommand(FormatBaseName(a.basename)))
	}

	// set program run command
	if a.runFunc != nil {
		// cmd.RunE将会在cmd.Execute()调用时时执行
		cmd.RunE = a.runCommand
	}

	var namedFlagSets cliflag.NamedFlagSets
	if a.options != nil {
		namedFlagSets = a.options.Flags()
		fs := cmd.Flags()
		for _, f := range namedFlagSets.FlagSets {
			fs.AddFlagSet(f)
		}
	}

	// 如果应用没有指定无版本, 则增加全局标志--version
	if !a.noVersion {
		verflag.AddFlags(namedFlagSets.FlagSet("global"))
	}

	// 如果应用没有指定无配置, 则增加全局标志--config
	if !a.noConfig {
		addConfigFlag(a.basename, namedFlagSets.FlagSet("global"))
	}

	// 设置默认全局标志--help
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), cmd.Name())
	// add new global flagset to cmd FlagSet
	cmd.Flags().AddFlagSet(namedFlagSets.FlagSet("global"))

	// 设置应用使用说明
	addCmdTemplate(&cmd, namedFlagSets)
	a.cmd = &cmd
}

// Run is used to launch the application.
func (a *App) Run() {
	// 包括1. 解析标志位 2. 执行初始化函数(如cobra的配置加载)等动作
	if err := a.cmd.Execute(); err != nil {
		fmt.Printf("%v %v\n", color.RedString("Error:"), err)
		os.Exit(1)
	}
}

// Command returns cobra command instance inside the application.
func (a *App) Command() *cobra.Command {
	return a.cmd
}

func (a *App) Flags() *pflag.FlagSet {
	return a.cmd.Flags()
}

// 应用执行命令逻辑.
func (a *App) runCommand(cmd *cobra.Command, args []string) error {
	if !a.noVersion {
		// display application version information
		verflag.PrintAndExitIfRequested()
	}

	if !a.noConfig {
		if err := loadConfig(cfgFile, a.basename); err != nil {
			return err
		}

		if err := viper.BindPFlags(cmd.Flags()); err != nil {
			return err
		}

		// 解析配置到选项
		// 注意:options中的字段必须要带有`mapstructure` tag才能正确解析!
		if err := viper.Unmarshal(a.options); err != nil {
			return fmt.Errorf("unmarshal config to options fail:%w", err)
		}

		// 调试用,用于打印viper加载的配置项，以及解析后的option结构
		// 在出现配置文件中的值没有作用于应用的选项时,移除注释进行调试
		// viper.Debug()
		if printableOptions, ok := a.options.(PrintableOptions); ok && !a.silence {
			log.Infof("viper ----> %v Config: `%s`", progressMessage, printableOptions.String())
		}
	}

	if !a.silence {
		printWorkingDir()

		// 打印应用运行时设置的标志位
		if !a.noPrintFlag {
			cliflag.PrintFlags(cmd.Flags())
		}

		log.Infof("%v Starting %s ...", progressMessage, a.name)
		if !a.noVersion {
			log.Infof("%v Version: `%s`", progressMessage, version.Get().ToJSON())
		}

		// 如果没有指定无配置, 则打印所用的配置文件路径
		if !a.noConfig {
			log.Infof("%v Config file used: `%s`", progressMessage, viper.ConfigFileUsed())
		}
	}

	if a.options != nil {
		// 补全应用选项参数并进行参数检测
		if err := a.applyOptionRules(); err != nil {
			return err
		}
	}

	// 运行应用真正的执行逻辑
	if a.runFunc != nil {
		return a.runFunc(a.basename)
	}

	return nil
}

func (a *App) applyOptionRules() error {
	if completableOptions, ok := a.options.(CompleteableOptions); ok {
		// 补全选项参数
		if err := completableOptions.Complete(); err != nil {
			return err
		}
	}

	// 检测选项参数是否合法
	if errs := a.options.Validate(); len(errs) != 0 {
		return errors.NewAggregate(errs...)
	}

	// 如果选项参数支持打印, 则打印应用最终的运行选项参数(命令行、配置等作用后的最终选项)
	if printableOptions, ok := a.options.(PrintableOptions); ok && !a.silence {
		log.Infof("%v Config: `%s`", progressMessage, printableOptions.String())
	}

	return nil
}

func printWorkingDir() {
	wd, _ := os.Getwd()
	log.Infof("%v WorkingDir: %s", progressMessage, wd)
}

func addCmdTemplate(cmd *cobra.Command, namedFlagSets cliflag.NamedFlagSets) {
	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := terminal.TerminalSize(cmd.OutOrStdout())
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), namedFlagSets, cols)

		return nil
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), namedFlagSets, cols)
	})
}
