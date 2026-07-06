package main

import (
	"github.com/spf13/pflag"

	"github.com/wangweihong/gotoolbox/pkg/json"
	"github.com/wangweihong/gotoolbox/pkg/log"
)

// Options runs an app server.
type Options struct {
	Log *log.Options `json:"log" mapstructure:"log"`
}

func NewOptions() *Options {
	return &Options{
		Log: log.NewOptions(),
	}
}

// ApplyTo applies the run options to the method receiver and returns self.
//func (o *Options) ApplyTo(c *server.Config) error {
//	return nil
//}

// Flags returns flags for a specific APIServer by section name.
func (o *Options) Flags() (fss pflag.FlagSet) {
	flagSet := pflag.NewFlagSet("log", pflag.ExitOnError)

	o.Log.AddFlags(flagSet)
	return fss
}

func (o *Options) String() string {
	data, _ := json.Marshal(o)

	return string(data)
}

// Complete set default Options.
func (o *Options) Complete() error {
	return nil
}
