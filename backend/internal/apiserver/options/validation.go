package options

import "fmt"

// Validate checks Options and return a slice of found errs.
func (o *Options) Validate() []error {
	var errs []error

	errs = append(errs, o.Log.Validate()...)
	errs = append(errs, o.GenericServerRunOptions.Validate()...)
	errs = append(errs, o.FeatureOptions.Validate()...)
	errs = append(errs, o.InsecureServing.Validate()...)
	errs = append(errs, o.SecureServing.Validate()...)
	//errs = append(errs, o.PostgresSQLOptions.Validate()...)
	errs = append(errs, o.DatabaseOptions.Validate()...)

	if !o.InsecureServing.Required && !o.SecureServing.Required {
		errs = append(errs, fmt.Errorf("--insecure.required and --secure.required must not set to `false` at sametime"))
	}
	if o.WorkflowRuntimeOptions != nil && o.WorkflowRuntimeOptions.Enabled && o.WorkflowRuntimeOptions.BaseURL == "" {
		errs = append(errs, fmt.Errorf("workflow-runtime.base-url is required when workflow runtime is enabled"))
	}

	return errs
}
