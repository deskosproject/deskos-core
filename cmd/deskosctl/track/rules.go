package track

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Rule says how to read a version out of an upstream release tag for one
// repository. The tag does not always carry the version unchanged:
// `extensions.gnome.org-v110` publishes version `110`, and `v0.17.2` publishes
// `0.17.2`. Nothing derives one from the other reliably, so the list is curated
// rather than guessed.
type Rule struct {
	// Match is the repository prefix of the release URL, for example
	// `github.com/deskosproject/deskos-core`.
	Match string `yaml:"match"`
	// VersionFromTag is removed from the front of the tag to obtain the version.
	VersionFromTag string `yaml:"version_from_tag"`
}

type ruleFile struct {
	Rules []Rule `yaml:"rules"`
}

// LoadRules reads the curated list. A file without rules is an error: an empty
// list would silently stop every pin from moving.
func LoadRules(path string) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f ruleFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(f.Rules) == 0 {
		return nil, fmt.Errorf("%s: no rules", path)
	}
	for i, r := range f.Rules {
		if r.Match == "" {
			return nil, fmt.Errorf("%s: rule %d has no match", path, i+1)
		}
	}
	return f.Rules, nil
}

// ruleFor finds the rule that owns a release URL.
func ruleFor(rules []Rule, raw string) (Rule, bool) {
	u, ok := releaseParts(raw)
	if !ok {
		return Rule{}, false
	}
	repo := "github.com/" + u.owner + "/" + u.repo
	for _, r := range rules {
		if strings.TrimSuffix(r.Match, "/") == repo {
			return r, true
		}
	}
	return Rule{}, false
}

// versionOf applies a rule to a tag.
func (r Rule) versionOf(tag string) string {
	return strings.TrimPrefix(tag, r.VersionFromTag)
}
