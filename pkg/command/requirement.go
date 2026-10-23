package command

import (
	"sort"
	"strings"

	"github.com/pasataleo/go-config/pkg/flags"
)

// requirements works out how each field's requirement reads in help and man
// pages, keyed by the same paths as meta: "required" for a field required on
// its own, "required: one of --port, --socket" for one in a oneof group, and
// nothing for a field that is not required.
//
// Groups are global, as flag names are, so a group's members are gathered
// from every path in meta, whichever struct or module they came from.
func requirements(meta map[string]map[string]interface{}) map[string]string {
	members := map[string][]string{}
	for _, sources := range meta {
		group, ok := sources["oneof"].(string)
		if !ok {
			continue
		}
		if label := memberLabel(sources); !contains(members[group], label) {
			members[group] = append(members[group], label)
		}
	}
	for _, labels := range members {
		sort.Strings(labels)
	}

	result := map[string]string{}
	for path, sources := range meta {
		if _, ok := sources["required"]; !ok {
			continue
		}
		if group, ok := sources["oneof"].(string); ok {
			result[path] = "required: one of " + strings.Join(members[group], ", ")
			continue
		}
		result[path] = "required"
	}
	return result
}

// memberLabel names a group member the way a user would supply it: by its
// flag, or its environment variable if it has no flag, or failing both by the
// name its required tag gives it.
func memberLabel(sources map[string]interface{}) string {
	if flag, ok := sources["flags"].(flags.Flag); ok {
		return "--" + flag.Name
	}
	if env, ok := sources["env"].(string); ok {
		return env
	}
	name, _ := sources["required"].(string)
	return name
}

func contains(labels []string, label string) bool {
	for _, l := range labels {
		if l == label {
			return true
		}
	}
	return false
}
