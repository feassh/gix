package config

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type ValueKind string

const (
	StringKind ValueKind = "string"
	BoolKind   ValueKind = "bool"
	IntKind    ValueKind = "int"
)

type KeySpec struct {
	Kind ValueKind
}

var Schema = map[string]KeySpec{
	"ai.provider":               {Kind: StringKind},
	"ai.model":                  {Kind: StringKind},
	"ai.base_url":               {Kind: StringKind},
	"ai.api_key":                {Kind: StringKind},
	"ai.api_key_env":            {Kind: StringKind},
	"ai.timeout":                {Kind: IntKind},
	"ai.language":               {Kind: StringKind},
	"ai.thinking":               {Kind: BoolKind},
	"commit.style":              {Kind: StringKind},
	"commit.language":           {Kind: StringKind},
	"commit.default_type":       {Kind: StringKind},
	"commit.default_scope":      {Kind: StringKind},
	"commit.with_body":          {Kind: BoolKind},
	"commit.confirm":            {Kind: BoolKind},
	"commit.max_subject_length": {Kind: IntKind},
	"push.default_remote":       {Kind: StringKind},
	"ui.color":                  {Kind: BoolKind},
	"ui.interactive":            {Kind: BoolKind},
	"project.name":              {Kind: StringKind},
	"project.main_branch":       {Kind: StringKind},
	"project.default_remote":    {Kind: StringKind},
	"project.upstream_remote":   {Kind: StringKind},
	"tag.enabled":               {Kind: BoolKind},
	"tag.prefix":                {Kind: StringKind},
	"tag.pattern":               {Kind: StringKind},
	"tag.current":               {Kind: StringKind},
	"tag.auto_increment":        {Kind: StringKind},
	"tag.annotated":             {Kind: BoolKind},
	"tag.push_after_create":     {Kind: BoolKind},
	"self_update.repo":          {Kind: StringKind},
	"self_update.base_url":      {Kind: StringKind},
}

type Values map[string]string

type Config struct {
	AI         AIConfig
	Commit     CommitConfig
	Push       PushConfig
	UI         UIConfig
	Project    ProjectConfig
	Tag        TagConfig
	SelfUpdate SelfUpdateConfig
}

type AIConfig struct {
	Provider string
	Model    string
	BaseURL  string
	APIKey   string
	Timeout  int
	Language string
	Thinking bool
}

type CommitConfig struct {
	Style            string
	Language         string
	DefaultType      string
	DefaultScope     string
	WithBody         bool
	Confirm          bool
	MaxSubjectLength int
}

type PushConfig struct {
	DefaultRemote string
}

type UIConfig struct {
	Color       bool
	Interactive bool
}

type ProjectConfig struct {
	Name           string
	MainBranch     string
	DefaultRemote  string
	UpstreamRemote string
}

type TagConfig struct {
	Enabled         bool
	Prefix          string
	Pattern         string
	Current         string
	AutoIncrement   string
	Annotated       bool
	PushAfterCreate bool
}

type SelfUpdateConfig struct {
	Repo    string
	BaseURL string
}

func DefaultValues() Values {
	return Values{
		"ai.provider":               "openai",
		"ai.model":                  "gpt-5-mini",
		"ai.base_url":               "https://api.openai.com/v1",
		"ai.api_key":                "",
		"ai.timeout":                "30",
		"ai.language":               "en",
		"ai.thinking":               "true",
		"commit.style":              "conventional",
		"commit.language":           "en",
		"commit.default_type":       "",
		"commit.default_scope":      "",
		"commit.with_body":          "false",
		"commit.confirm":            "true",
		"commit.max_subject_length": "72",
		"push.default_remote":       "origin",
		"ui.color":                  "true",
		"ui.interactive":            "true",
		"project.name":              "",
		"project.main_branch":       "main",
		"project.default_remote":    "origin",
		"project.upstream_remote":   "upstream",
		"tag.enabled":               "true",
		"tag.prefix":                "v",
		"tag.pattern":               "semver",
		"tag.current":               "",
		"tag.auto_increment":        "patch",
		"tag.annotated":             "true",
		"tag.push_after_create":     "false",
		"self_update.repo":          "",
		"self_update.base_url":      "https://api.github.com",
	}
}

func (v Values) Clone() Values {
	out := make(Values, len(v))
	for key, value := range v {
		out[key] = value
	}
	return out
}

func (v Values) Merge(other Values) Values {
	out := v.Clone()
	for key, value := range other {
		out[key] = value
	}
	return out
}

func (v Values) Set(path string, value string) error {
	key := normalizeKey(path)
	spec, ok := Schema[key]
	if !ok {
		return fmt.Errorf("unknown config key %q", path)
	}
	if _, err := normalizeValue(spec.Kind, value); err != nil {
		return err
	}
	v[key] = strings.TrimSpace(value)
	return nil
}

func (v Values) Get(path string) (string, bool) {
	value, ok := v[normalizeKey(path)]
	return value, ok
}

func (v Values) SortedKeys() []string {
	keys := make([]string, 0, len(v))
	for key := range v {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (v Values) ToConfig() (Config, error) {
	cfg := Config{}
	var err error
	cfg.AI.Provider = v["ai.provider"]
	cfg.AI.Model = v["ai.model"]
	cfg.AI.BaseURL = v["ai.base_url"]
	cfg.AI.APIKey = firstNonEmptyValue(v["ai.api_key"], v["ai.api_key_env"])
	cfg.AI.Language = v["ai.language"]
	if cfg.AI.Timeout, err = strconv.Atoi(v["ai.timeout"]); err != nil {
		return Config{}, fmt.Errorf("invalid ai.timeout: %w", err)
	}
	if cfg.AI.Thinking, err = strconv.ParseBool(v["ai.thinking"]); err != nil {
		return Config{}, fmt.Errorf("invalid ai.thinking: %w", err)
	}
	cfg.Commit.Style = v["commit.style"]
	cfg.Commit.Language = v["commit.language"]
	cfg.Commit.DefaultType = v["commit.default_type"]
	cfg.Commit.DefaultScope = v["commit.default_scope"]
	if cfg.Commit.WithBody, err = strconv.ParseBool(v["commit.with_body"]); err != nil {
		return Config{}, fmt.Errorf("invalid commit.with_body: %w", err)
	}
	if cfg.Commit.Confirm, err = strconv.ParseBool(v["commit.confirm"]); err != nil {
		return Config{}, fmt.Errorf("invalid commit.confirm: %w", err)
	}
	if cfg.Commit.MaxSubjectLength, err = strconv.Atoi(v["commit.max_subject_length"]); err != nil {
		return Config{}, fmt.Errorf("invalid commit.max_subject_length: %w", err)
	}
	cfg.Push.DefaultRemote = v["push.default_remote"]
	if cfg.UI.Color, err = strconv.ParseBool(v["ui.color"]); err != nil {
		return Config{}, fmt.Errorf("invalid ui.color: %w", err)
	}
	if cfg.UI.Interactive, err = strconv.ParseBool(v["ui.interactive"]); err != nil {
		return Config{}, fmt.Errorf("invalid ui.interactive: %w", err)
	}
	cfg.Project.Name = v["project.name"]
	cfg.Project.MainBranch = v["project.main_branch"]
	cfg.Project.DefaultRemote = v["project.default_remote"]
	cfg.Project.UpstreamRemote = v["project.upstream_remote"]
	if cfg.Tag.Enabled, err = strconv.ParseBool(v["tag.enabled"]); err != nil {
		return Config{}, fmt.Errorf("invalid tag.enabled: %w", err)
	}
	cfg.Tag.Prefix = v["tag.prefix"]
	cfg.Tag.Pattern = v["tag.pattern"]
	cfg.Tag.Current = v["tag.current"]
	cfg.Tag.AutoIncrement = v["tag.auto_increment"]
	if cfg.Tag.Annotated, err = strconv.ParseBool(v["tag.annotated"]); err != nil {
		return Config{}, fmt.Errorf("invalid tag.annotated: %w", err)
	}
	if cfg.Tag.PushAfterCreate, err = strconv.ParseBool(v["tag.push_after_create"]); err != nil {
		return Config{}, fmt.Errorf("invalid tag.push_after_create: %w", err)
	}
	cfg.SelfUpdate.Repo = v["self_update.repo"]
	cfg.SelfUpdate.BaseURL = v["self_update.base_url"]
	return cfg, nil
}

func normalizeKey(path string) string {
	return strings.ToLower(strings.TrimSpace(path))
}

func firstNonEmptyValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeValue(kind ValueKind, value string) (string, error) {
	raw := strings.TrimSpace(value)
	switch kind {
	case StringKind:
		return raw, nil
	case BoolKind:
		if _, err := strconv.ParseBool(raw); err != nil {
			return "", fmt.Errorf("expected boolean value for %q", raw)
		}
		return raw, nil
	case IntKind:
		if _, err := strconv.Atoi(raw); err != nil {
			return "", fmt.Errorf("expected integer value for %q", raw)
		}
		return raw, nil
	default:
		return "", fmt.Errorf("unsupported value type")
	}
}
