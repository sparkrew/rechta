package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ScopeMetadata           = "metadata"
	ScopeActions            = "actions"
	ScopeAttestations       = "attestations"
	ScopeChecks             = "checks"
	ScopeContents           = "contents"
	ScopeDeployments        = "deployments"
	ScopeIDToken            = "id-token"
	ScopeIssues             = "issues"
	ScopeDiscussions        = "discussions"
	ScopePackages           = "packages"
	ScopePages              = "pages"
	ScopePullRequests       = "pull-requests"
	ScopeRepositoryProjects = "repository-projects"
	ScopeSecurityEvents     = "security-events"
	ScopeStatuses           = "statuses"

	PermissionRead  = "read"
	PermissionWrite = "write"
	PermissionNone  = "none"
)

var AllScopes = []string{
	ScopeMetadata,
	ScopeActions,
	ScopeAttestations,
	ScopeChecks,
	ScopeContents,
	ScopeDeployments,
	ScopeIDToken,
	ScopeIssues,
	ScopeDiscussions,
	ScopePackages,
	ScopePages,
	ScopePullRequests,
	ScopeRepositoryProjects,
	ScopeSecurityEvents,
	ScopeStatuses,
}

const AllSecrets = "*ALL"

// ---------------------------------------------------------------------------
// Collection types
// ---------------------------------------------------------------------------

type Inputs []Input
type Outputs []Output
type Envs []Env
type Steps []Step
type Permissions []Permission
type Events []Event
type JobEnvironments []JobEnvironment
type Jobs []Job
type JobSecrets []JobSecret

type Secrets = Inputs
type With = Envs
type JobRunsOn StringList
type StringList []string

// ---------------------------------------------------------------------------
// Leaf types
// ---------------------------------------------------------------------------

type Input struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Type        string `json:"type"`
}

type Output struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value"`
}

type Env struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Permission struct {
	Scope      string `json:"scope"`
	Permission string `json:"permission"`
}

type JobSecret struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type JobContainer struct {
	Image string `json:"image"`
}

type JobEnvironment struct {
	Name string `json:"name"`
	Url  string `json:"url,omitempty"`
}

type Strategy struct {
	Matrix map[string]StringList `json:"matrix,omitempty" yaml:"matrix"`
}

// ---------------------------------------------------------------------------
// Step
// ---------------------------------------------------------------------------

type Step struct {
	ID               string `json:"id,omitempty"`
	Name             string `json:"name,omitempty"`
	If               string `json:"if,omitempty"`
	Env              Envs   `json:"env,omitempty"`
	Uses             string `json:"uses,omitempty"`
	Shell            string `json:"shell,omitempty"`
	Run              string `json:"run,omitempty" yaml:"run"`
	WorkingDirectory string `json:"working_directory,omitempty" yaml:"working-directory"`
	With             With   `json:"with,omitempty"`
	WithRef          string `json:"with_ref,omitempty" yaml:"-"`
	WithScript       string `json:"with_script,omitempty" yaml:"-"`
	Line             int    `json:"line" yaml:"-"`
	Action           string `json:"action,omitempty" yaml:"-"`

	Lines map[string]int `json:"lines" yaml:"-"`
}

// ---------------------------------------------------------------------------
// Event
// ---------------------------------------------------------------------------

type Event struct {
	Name           string     `json:"name"`
	Types          StringList `json:"types,omitempty"`
	Branches       StringList `json:"branches,omitempty"`
	BranchesIgnore StringList `json:"branches_ignore,omitempty"`
	Paths          StringList `json:"paths,omitempty"`
	PathsIgnore    StringList `json:"paths_ignore,omitempty"`
	Tags           StringList `json:"tags,omitempty"`
	TagsIgnore     StringList `json:"tags_ignore,omitempty"`
	Cron           StringList `json:"cron,omitempty"`
	Inputs         Inputs     `json:"inputs,omitempty"`
	Outputs        Outputs    `json:"outputs,omitempty"`
	Secrets        Secrets    `json:"secrets,omitempty"`
	Workflows      StringList `json:"workflows,omitempty"`
}

// ---------------------------------------------------------------------------
// Job
// ---------------------------------------------------------------------------

type Job struct {
	ID          string          `json:"id"`
	Name        string          `json:"name,omitempty"`
	Uses        string          `json:"uses,omitempty"`
	Secrets     JobSecrets      `json:"secrets,omitempty"`
	With        With            `json:"with,omitempty"`
	Permissions Permissions     `json:"permissions"`
	Needs       StringList      `json:"needs,omitempty"`
	If          string          `json:"if,omitempty"`
	RunsOn      JobRunsOn       `json:"runs_on" yaml:"runs-on"`
	Container   JobContainer    `json:"container"`
	Environment JobEnvironments `json:"environment,omitempty"`
	Outputs     Envs            `json:"outputs,omitempty"`
	Env         Envs            `json:"env,omitempty"`
	Steps       Steps           `json:"steps"`
	Strategy    Strategy        `json:"strategy,omitempty" yaml:"strategy"`
	Line        int             `json:"line" yaml:"-"`

	Lines map[string]int `json:"lines" yaml:"-"`
}

// ---------------------------------------------------------------------------
// Workflow (top-level)
// ---------------------------------------------------------------------------

type Workflow struct {
	Path        string      `json:"path" yaml:"-"`
	Name        string      `json:"name"`
	Events      Events      `json:"events" yaml:"on"`
	Permissions Permissions `json:"permissions"`
	Env         Envs        `json:"env,omitempty"`
	Jobs        Jobs        `json:"jobs"`
}

func (w Workflow) IsValid() bool {
	return len(w.Jobs) > 0 && len(w.Events) > 0
}

// ---------------------------------------------------------------------------
// Metadata (action.yml / action.yaml)
// ---------------------------------------------------------------------------

type Metadata struct {
	Path        string  `json:"path"`
	Name        string  `json:"name" yaml:"name"`
	Description string  `json:"description" yaml:"description"`
	Author      string  `json:"author" yaml:"author"`
	Inputs      Inputs  `json:"inputs"`
	Outputs     Outputs `json:"outputs"`
	Runs        struct {
		Using          string   `json:"using"`
		Main           string   `json:"main"`
		Pre            string   `json:"pre"`
		PreIf          string   `json:"pre-if"`
		Post           string   `json:"post"`
		PostIf         string   `json:"post-if"`
		Steps          Steps    `json:"steps"`
		Image          string   `json:"image"`
		Entrypoint     string   `json:"entrypoint"`
		PreEntrypoint  string   `json:"pre-entrypoint"`
		PostEntrypoint string   `json:"post-entrypoint"`
		Args           []string `json:"args"`
	} `json:"runs"`
}

func (m Metadata) IsValid() bool {
	return m.Runs.Using != ""
}

// ===========================================================================
// UnmarshalYAML implementations
// ===========================================================================

// StringList: "value" | ["a","b"]
func (o *StringList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*o = []string{node.Value}
		return nil
	}

	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("invalid yaml node type %v for string list", node.Kind)
	}

	l := make([]string, len(node.Content))
	if err := node.Decode(&l); err != nil {
		return err
	}

	*o = l
	return nil
}

// Events: on: push | on: [push, pull_request] | on: {push: {branches: [main]}}
func (o *Events) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		*o = Events{{Name: node.Value}}
	case yaml.SequenceNode:
		*o = make(Events, 0, len(node.Content))
		for _, item := range node.Content {
			*o = append(*o, Event{Name: item.Value})
		}
	case yaml.MappingNode:
		*o = make(Events, 0, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			name := node.Content[i].Value
			value := node.Content[i+1]
			event := Event{Name: name}

			if name == "schedule" {
				var crons []struct {
					Cron string `json:"cron"`
				}
				if err := value.Decode(&crons); err != nil {
					return err
				}
				for _, c := range crons {
					if c.Cron == "" {
						return fmt.Errorf("invalid cron object")
					}
					event.Cron = append(event.Cron, c.Cron)
				}
			} else {
				if err := value.Decode(&event); err != nil {
					return err
				}
			}
			*o = append(*o, event)
		}
	}
	return nil
}

// Jobs: mapping of job-id → job definition
func (o *Jobs) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("invalid yaml node type for jobs")
	}

	*o = make(Jobs, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		name := node.Content[i].Value
		value := node.Content[i+1]

		job := Job{
			ID:   name,
			Line: node.Content[i].Line,
			Lines: map[string]int{
				"start": node.Content[i].Line,
			},
		}
		if err := value.Decode(&job); err != nil {
			return err
		}

		for j := 0; j < len(value.Content); j += 2 {
			key := value.Content[j].Value
			v := value.Content[j+1]
			switch key {
			case "runs-on":
				job.Lines["runs_on"] = v.Line
			case "if":
				job.Lines[key] = v.Line
			}
		}

		*o = append(*o, job)
	}
	return nil
}

// JobSecrets: "inherit" | {TOKEN: "${{ secrets.TOKEN }}"}
func (o *JobSecrets) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode && node.Value == "inherit" {
		*o = JobSecrets{{Name: AllSecrets, Value: "inherit"}}
		return nil
	}

	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("invalid yaml node type for secrets")
	}

	for i := 0; i < len(node.Content); i += 2 {
		name := node.Content[i].Value
		value := node.Content[i+1].Value
		*o = append(*o, JobSecret{Name: name, Value: value})
	}
	return nil
}

// Inputs: mapping of input-name → input definition
func (o *Inputs) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("invalid yaml node type for inputs")
	}

	for i := 0; i < len(node.Content); i += 2 {
		name := node.Content[i].Value
		value := node.Content[i+1]
		input := Input{Name: name}
		if err := value.Decode(&input); err != nil {
			return err
		}
		*o = append(*o, input)
	}
	return nil
}

// Outputs: mapping of output-name → output definition (scalar value or mapping)
func (o *Outputs) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("invalid yaml node type for outputs")
	}

	for i := 0; i < len(node.Content); i += 2 {
		name := node.Content[i].Value
		value := node.Content[i+1]

		if value.Kind == yaml.ScalarNode {
			*o = append(*o, Output{Name: name, Value: value.Value})
		} else if value.Kind == yaml.MappingNode {
			output := Output{Name: name}
			if err := value.Decode(&output); err != nil {
				return err
			}
			*o = append(*o, output)
		}
	}
	return nil
}

// Envs: {KEY: "value"} or expression scalar "${{ ... }}"
func (o *Envs) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		if len(node.Value) > 0 && node.Value[0] == '$' {
			*o = Envs{{Value: node.Value}}
			return nil
		}
	}

	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("invalid yaml node type for env")
	}

	for i := 0; i < len(node.Content); i += 2 {
		name := node.Content[i].Value
		value := node.Content[i+1].Value
		*o = append(*o, Env{name, value})
	}
	return nil
}

// Step: captures line info and extracts with.ref / with.script / action from uses
func (o *Step) UnmarshalYAML(node *yaml.Node) error {
	type Alias Step
	t := Alias{
		Line:  node.Line,
		Lines: map[string]int{"start": node.Line},
	}
	if err := node.Decode(&t); err != nil {
		return err
	}

	*o = Step(t)

	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		value := node.Content[i+1]

		switch key {
		case "uses", "run", "if":
			o.Lines[key] = value.Line
		case "with":
			if value.Kind != yaml.MappingNode {
				continue
			}
			for j := 0; j < len(value.Content); j += 2 {
				name := value.Content[j].Value
				arg := value.Content[j+1]
				switch name {
				case "ref":
					o.Lines["with_ref"] = arg.Line
					o.WithRef = arg.Value
				case "script":
					o.Lines["with_script"] = arg.Line
					o.WithScript = arg.Value
				}
			}
		}
	}

	o.Action, _, _ = strings.Cut(o.Uses, "@")
	return nil
}

// Permissions: "read-all" | "write-all" | {contents: read, ...}
func (o *Permissions) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		var permission string
		switch node.Value {
		case "write-all":
			permission = PermissionWrite
		case "read-all":
			permission = PermissionRead
		default:
			return fmt.Errorf("invalid permission %s", node.Value)
		}

		*o = make(Permissions, 0, len(AllScopes))
		for _, scope := range AllScopes {
			*o = append(*o, Permission{scope, permission})
		}
		return nil
	}

	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("invalid yaml node type for permissions")
	}

	*o = make(Permissions, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		scope := node.Content[i].Value
		permission := node.Content[i+1].Value
		*o = append(*o, Permission{scope, permission})
	}
	return nil
}

// JobRunsOn: "ubuntu-latest" | ["self-hosted","linux"] | {group: ..., labels: [...]}
func (o *JobRunsOn) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.SequenceNode || node.Kind == yaml.ScalarNode {
		var runsOn StringList
		if err := node.Decode(&runsOn); err != nil {
			return err
		}
		*o = JobRunsOn(runsOn)
	}

	if node.Kind == yaml.MappingNode {
		type RunsOn struct {
			Group  StringList `json:"group"`
			Labels StringList `json:"labels"`
		}
		var runsOn RunsOn
		if err := node.Decode(&runsOn); err != nil {
			return err
		}
		for _, group := range runsOn.Group {
			if group == "" {
				return fmt.Errorf("unexpected empty group")
			}
			*o = append(*o, fmt.Sprintf("group:%s", group))
		}
		for _, label := range runsOn.Labels {
			if label == "" {
				return fmt.Errorf("unexpected empty label")
			}
			*o = append(*o, fmt.Sprintf("label:%s", label))
		}
	}
	return nil
}

// JobContainer: "alpine:latest" | {image: "alpine:latest"}
func (o *JobContainer) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		o.Image = node.Value
		return nil
	}

	type container JobContainer
	var c container
	if err := node.Decode(&c); err != nil {
		return err
	}
	*o = JobContainer(c)
	return nil
}

// JobEnvironments: "production" | {name: "prod", url: "..."}
func (o *JobEnvironments) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*o = JobEnvironments{{Name: node.Value}}
		return nil
	}

	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("invalid yaml node type for environment")
	}

	var env JobEnvironment
	if err := node.Decode(&env); err != nil {
		return err
	}
	*o = JobEnvironments{env}
	return nil
}

// Strategy: extracts the matrix dimensions from the strategy block
func (o *Strategy) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return errors.New("invalid yaml node type for strategy")
	}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		value := node.Content[i+1]
		if key != "matrix" {
			continue
		}
		if value.Kind != yaml.MappingNode {
			return errors.New("matrix must be a mapping")
		}
		m := make(map[string]StringList, len(value.Content)/2)
		for j := 0; j < len(value.Content); j += 2 {
			dim := value.Content[j].Value
			listNode := value.Content[j+1]
			if listNode.Kind != yaml.SequenceNode {
				return fmt.Errorf("matrix.%s must be a sequence", dim)
			}
			var items StringList
			for _, item := range listNode.Content {
				switch item.Kind {
				case yaml.ScalarNode:
					items = append(items, item.Value)
				case yaml.MappingNode:
					var obj map[string]interface{}
					if err := item.Decode(&obj); err != nil {
						return fmt.Errorf("failed to decode matrix item: %w", err)
					}
					b, err := json.Marshal(obj)
					if err != nil {
						return fmt.Errorf("failed to marshal matrix item: %w", err)
					}
					items = append(items, string(b))
				default:
					return fmt.Errorf("unsupported node kind %v in matrix.%s", item.Kind, dim)
				}
			}
			m[dim] = items
		}
		o.Matrix = m
	}
	return nil
}
