package gitcode

// Action represents the root of action.yml
type Action struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description"`
	Inputs      map[string]*Input  `yaml:"inputs,omitempty"`
	Outputs     map[string]*Output `yaml:"outputs,omitempty"`
	Runs        Runs               `yaml:"runs"`
}

type Input struct {
	Description string `yaml:"description"`
	Required    bool   `yaml:"required,omitempty"`
	Default     string `yaml:"default,omitempty"`
}

type Output struct {
	Description string `yaml:"description"`
	Value       string `yaml:"value,omitempty"`
}

type Runs struct {
	Using string `yaml:"using"`
	Steps []Step `yaml:"steps,omitempty"`
}

type Step struct {
	Name  string            `yaml:"name"`
	Shell string            `yaml:"shell"`
	Run   string            `yaml:"run"`
	Uses  string            `yaml:"uses,omitempty"`
	With  map[string]string `yaml:"with,omitempty"`
	// Note: This simplified version assumes only shell/run steps.
	// For full GitHub Actions compatibility, you'd also support `uses:`, `with:`, etc.
}
