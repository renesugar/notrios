package docaudit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var headingLine = regexp.MustCompile(`^(#{1,3})\s+(.+?)\s*$`)

func topicForPath(path string) string {
	value := strings.TrimSuffix(strings.TrimPrefix(filepath.ToSlash(path), "docs/"), ".md")
	return strings.ReplaceAll(value, "/", "-")
}

func slug(value string) string {
	value = strings.ToLower(value)
	var out strings.Builder
	dash := false
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			if dash && out.Len() > 0 {
				out.WriteByte('-')
			}
			out.WriteRune(char)
			dash = false
		} else {
			dash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

func scanExecutableExamples(root string, inventory Inventory) ([]ExampleCandidate, error) {
	var candidates []ExampleCandidate
	for _, document := range inventory.Documents {
		bytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(document.Path)))
		if err != nil {
			return nil, err
		}
		section := ""
		ordinal := make(map[string]int)
		lines := strings.Split(string(bytes), "\n")
		for index := 0; index < len(lines); index++ {
			if match := headingLine.FindStringSubmatch(lines[index]); match != nil {
				section = slug(match[2])
				continue
			}
			trimmed := strings.TrimSpace(lines[index])
			if !strings.HasPrefix(trimmed, "```") && !strings.HasPrefix(trimmed, "~~~") {
				continue
			}
			marker := trimmed[:3]
			language := ""
			if fields := strings.Fields(strings.TrimSpace(trimmed[3:])); len(fields) > 0 {
				language = strings.ToLower(fields[0])
			}
			start := index + 1
			for index++; index < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[index]), marker); index++ {
			}
			body := strings.Join(lines[start:index], "\n")
			if !isExecutableFence(language, body) {
				continue
			}
			ordinal[section]++
			digest := sha256.Sum256([]byte(body))
			idBase := strings.TrimSuffix(strings.TrimPrefix(document.Path, "docs/"), ".md")
			idBase = strings.ReplaceAll(idBase, "/", "-")
			candidates = append(candidates, ExampleCandidate{
				ID:   fmt.Sprintf("%s-%s-example-%d", idBase, section, ordinal[section]),
				Path: document.Path, Section: section, Language: language,
				SHA256: hex.EncodeToString(digest[:]), Body: body,
			})
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	return candidates, nil
}

func isExecutableFence(language, body string) bool {
	switch language {
	case "bash", "sh", "shell", "console", "yaml", "yml", "toml", "http":
		return true
	case "json":
		return strings.Contains(body, `"jsonrpc"`) || strings.Contains(body, `"method"`) || strings.Contains(body, `"server"`)
	}
	trimmed := strings.TrimSpace(body)
	prefixes := []string{"notriosctl ", "notriosd ", "curl ", "go run ", "npm ", "make ", "$ notrios", "$ curl", "GET /api/", "POST /api/", "PUT /api/", "PATCH /api/", "DELETE /api/"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}
