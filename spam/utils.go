package spam

import "strings"


func onlyAPIs(m map[string]string) []string {
	apis := make([]string, 0, len(m))
	for k := range m {
		if !strings.Contains(strings.ToLower(k), "api") {
			continue
		}
		apis = append(apis, k)
	}
	return apis
}
