package ai

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultEvidenceBudgetChars = 16000
	maxRenderedFiles           = 12
	maxRenderedHunks           = 18
	maxRenderedHunksPerFile    = 3
	maxSnippetLines            = 8
	maxSnippetChars            = 480
)

type CommitEvidenceInput struct {
	Files      []string
	NameStatus string
	NumStat    string
	Summary    string
	DirStat    string
	Diff       string
}

type CommitEvidence struct {
	Overview EvidenceOverview
	Modules  []ModuleWeight
	Files    []FileEvidence
}

type EvidenceOverview struct {
	FileCount  int
	Additions  int
	Deletions  int
	Renames    int
	Created    int
	Deleted    int
	Copies     int
	Modified   int
	BinaryLike int
}

type ModuleWeight struct {
	Path    string
	Percent float64
}

type FileEvidence struct {
	Path         string
	PreviousPath string
	Status       string
	Kind         string
	Additions    int
	Deletions    int
	Score        int
	Hunks        []HunkEvidence
}

type HunkEvidence struct {
	Header  string
	Snippet string
	Score   int
}

type promptWriter struct {
	limit int
	b     strings.Builder
}

func BuildCommitEvidence(input CommitEvidenceInput) CommitEvidence {
	filesByPath := make(map[string]*FileEvidence)
	ensureFile := func(path string) *FileEvidence {
		path = normalizePath(path)
		if path == "" {
			path = "(unknown)"
		}
		if existing, ok := filesByPath[path]; ok {
			return existing
		}
		file := &FileEvidence{Path: path}
		filesByPath[path] = file
		return file
	}

	for _, path := range input.Files {
		ensureFile(path)
	}

	overview := EvidenceOverview{}

	for _, line := range splitLines(input.NameStatus) {
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		statusToken := strings.TrimSpace(parts[0])
		statusCode := firstStatusCode(statusToken)
		if statusCode == "" {
			continue
		}
		path := parts[len(parts)-1]
		previousPath := ""
		if statusCode == "R" || statusCode == "C" {
			if len(parts) >= 3 {
				previousPath = parts[1]
				path = parts[2]
			}
		}
		file := ensureFile(path)
		file.Status = statusCode
		if previousPath != "" {
			file.PreviousPath = normalizePath(previousPath)
		}
		switch statusCode {
		case "A":
			overview.Created++
		case "D":
			overview.Deleted++
		case "R":
			overview.Renames++
		case "C":
			overview.Copies++
		default:
			overview.Modified++
		}
	}

	for _, line := range splitLines(input.NumStat) {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		path := normalizeStatPath(parts[2])
		file := ensureFile(path)
		add, addOK := parseNumStatValue(parts[0])
		del, delOK := parseNumStatValue(parts[1])
		if addOK {
			file.Additions = add
			overview.Additions += add
		}
		if delOK {
			file.Deletions = del
			overview.Deletions += del
		}
		if !addOK || !delOK {
			overview.BinaryLike++
		}
	}

	if strings.TrimSpace(input.NameStatus) == "" {
		for _, line := range splitLines(input.Summary) {
			lower := strings.ToLower(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(lower, "create mode"):
				overview.Created++
			case strings.HasPrefix(lower, "delete mode"):
				overview.Deleted++
			case strings.HasPrefix(lower, "rename"):
				overview.Renames++
			case strings.HasPrefix(lower, "copy"):
				overview.Copies++
			}
		}
	}

	modules := parseDirStat(input.DirStat)
	parseCompactDiff(input.Diff, ensureFile)

	files := make([]FileEvidence, 0, len(filesByPath))
	for _, file := range filesByPath {
		file.Kind = classifyPath(file.Path)
		if file.Status == "" {
			if file.Additions == 0 && file.Deletions == 0 && file.PreviousPath != "" {
				file.Status = "R"
			} else {
				file.Status = "M"
			}
		}
		if file.Additions == 0 && file.Deletions == 0 {
			added, deleted := estimateLineChanges(file.Hunks)
			file.Additions += added
			file.Deletions += deleted
		}
		file.Score = scoreFile(*file)
		if len(file.Hunks) > 1 {
			sort.SliceStable(file.Hunks, func(i, j int) bool {
				if file.Hunks[i].Score == file.Hunks[j].Score {
					return file.Hunks[i].Header < file.Hunks[j].Header
				}
				return file.Hunks[i].Score > file.Hunks[j].Score
			})
		}
		files = append(files, *file)
	}

	if len(modules) == 0 {
		modules = deriveModules(files)
	}

	overview.FileCount = len(files)
	sort.SliceStable(files, func(i, j int) bool {
		if files[i].Score == files[j].Score {
			return files[i].Path < files[j].Path
		}
		return files[i].Score > files[j].Score
	})

	return CommitEvidence{
		Overview: normalizeOverview(overview),
		Modules:  modules,
		Files:    files,
	}
}

func (e CommitEvidence) Empty() bool {
	return e.Overview.FileCount == 0 && len(e.Files) == 0 && len(e.Modules) == 0
}

func (e CommitEvidence) DominantScope() string {
	for _, module := range e.Modules {
		if scope := scopeCandidate(module.Path); scope != "" {
			return scope
		}
	}
	for _, file := range e.Files {
		if scope := scopeCandidate(file.Path); scope != "" {
			return scope
		}
	}
	return ""
}

func (e CommitEvidence) KindCounts() map[string]int {
	counts := make(map[string]int)
	for _, file := range e.Files {
		counts[file.Kind]++
	}
	return counts
}

func (e CommitEvidence) Prompt(maxChars int) string {
	if maxChars <= 0 {
		maxChars = defaultEvidenceBudgetChars
	}
	if e.Empty() {
		return ""
	}

	writer := &promptWriter{limit: maxChars}
	truncated := false

	writer.line("Compressed commit evidence:")
	writer.line(fmt.Sprintf("- files: %d", e.Overview.FileCount))
	writer.line(fmt.Sprintf("- line changes: +%d -%d", e.Overview.Additions, e.Overview.Deletions))
	if ops := e.operationSummary(); ops != "" {
		writer.line("- operations: " + ops)
	}

	if len(e.Modules) > 0 {
		writer.line("Dominant areas:")
		for i, module := range e.Modules {
			if i >= 3 {
				truncated = true
				break
			}
			if !writer.line(fmt.Sprintf("- %s (%.1f%%)", module.Path, module.Percent)) {
				truncated = true
				break
			}
		}
	}

	writer.line("Changed files:")
	shownFiles := 0
	shownHunks := 0
	for _, file := range e.Files {
		if shownFiles >= maxRenderedFiles {
			truncated = true
			break
		}
		if !writer.line(file.promptHeader()) {
			truncated = true
			break
		}
		shownFiles++

		if len(file.Hunks) == 0 {
			continue
		}
		renderedForFile := 0
		for _, hunk := range file.Hunks {
			if renderedForFile >= maxRenderedHunksPerFile || shownHunks >= maxRenderedHunks {
				truncated = true
				break
			}
			if hunk.Header != "" && !writer.line("  - "+hunk.Header) {
				truncated = true
				break
			}
			if hunk.Snippet == "" {
				continue
			}
			ok := true
			for _, line := range strings.Split(hunk.Snippet, "\n") {
				if !writer.line("    " + line) {
					ok = false
					break
				}
			}
			if !ok {
				truncated = true
				break
			}
			renderedForFile++
			shownHunks++
		}
		if truncated && (renderedForFile >= maxRenderedHunksPerFile || shownHunks >= maxRenderedHunks) {
			continue
		}
		if truncated {
			break
		}
	}

	if truncated {
		writer.line("Note: low-signal files or extra hunks were omitted to stay within context. Keep the message broad if intent is ambiguous.")
	}

	return strings.TrimSpace(writer.b.String())
}

func (e CommitEvidence) operationSummary() string {
	parts := make([]string, 0, 4)
	if e.Overview.Created > 0 {
		parts = append(parts, pluralize(e.Overview.Created, "new file", "new files"))
	}
	if e.Overview.Deleted > 0 {
		parts = append(parts, pluralize(e.Overview.Deleted, "deleted file", "deleted files"))
	}
	if e.Overview.Renames > 0 {
		parts = append(parts, pluralize(e.Overview.Renames, "rename", "renames"))
	}
	if e.Overview.Copies > 0 {
		parts = append(parts, pluralize(e.Overview.Copies, "copy", "copies"))
	}
	if e.Overview.BinaryLike > 0 {
		parts = append(parts, pluralize(e.Overview.BinaryLike, "binary-like file", "binary-like files"))
	}
	return strings.Join(parts, ", ")
}

func (f FileEvidence) promptHeader() string {
	parts := []string{fmt.Sprintf("- %s", f.Path)}
	meta := []string{strings.ToUpper(f.Status)}
	if f.Kind != "" {
		meta = append(meta, f.Kind)
	}
	if f.Additions > 0 || f.Deletions > 0 {
		meta = append(meta, fmt.Sprintf("+%d -%d", f.Additions, f.Deletions))
	}
	if f.PreviousPath != "" {
		meta = append(meta, "from "+f.PreviousPath)
	}
	return parts[0] + " [" + strings.Join(meta, " | ") + "]"
}

func (w *promptWriter) line(value string) bool {
	add := len(value) + 1
	if w.b.Len()+add > w.limit {
		return false
	}
	w.b.WriteString(value)
	w.b.WriteByte('\n')
	return true
}

func parseCompactDiff(diff string, ensureFile func(string) *FileEvidence) {
	var currentFile *FileEvidence
	var currentHeader string
	var currentLines []string

	flushHunk := func() {
		if currentFile == nil || currentHeader == "" {
			currentHeader = ""
			currentLines = nil
			return
		}
		hunk := buildHunkEvidence(currentHeader, currentLines)
		if hunk.Snippet != "" || hunk.Header != "" {
			currentFile.Hunks = append(currentFile.Hunks, hunk)
		}
		currentHeader = ""
		currentLines = nil
	}

	for _, raw := range strings.Split(strings.ReplaceAll(diff, "\r\n", "\n"), "\n") {
		line := strings.TrimRight(raw, "\r")
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flushHunk()
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				currentFile = ensureFile(normalizeGitDiffPath(parts[3]))
			}
		case strings.HasPrefix(line, "rename from "):
			if currentFile != nil {
				currentFile.PreviousPath = normalizePath(strings.TrimSpace(strings.TrimPrefix(line, "rename from ")))
				currentFile.Status = "R"
			}
		case strings.HasPrefix(line, "rename to "):
			flushHunk()
			currentFile = ensureFile(strings.TrimSpace(strings.TrimPrefix(line, "rename to ")))
		case strings.HasPrefix(line, "+++ b/"):
			flushHunk()
			currentFile = ensureFile(strings.TrimSpace(strings.TrimPrefix(line, "+++ b/")))
		case strings.HasPrefix(line, "+++ /dev/null"):
			flushHunk()
		case strings.HasPrefix(line, "@@"):
			flushHunk()
			currentHeader = strings.TrimSpace(line)
		case strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-"):
			if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
				continue
			}
			if currentHeader == "" {
				continue
			}
			trimmed := trimChangedLine(line)
			if trimmed != "" {
				currentLines = append(currentLines, trimmed)
			}
		}
	}
	flushHunk()
}

func buildHunkEvidence(header string, lines []string) HunkEvidence {
	snippetLines := make([]string, 0, len(lines))
	score := scoreHeader(header)
	for _, line := range lines {
		if line == "" {
			continue
		}
		snippetLines = append(snippetLines, line)
		score += scoreChangedLine(line)
		if len(snippetLines) >= maxSnippetLines {
			break
		}
	}
	snippet := strings.Join(snippetLines, "\n")
	if len(snippet) > maxSnippetChars {
		snippet = snippet[:maxSnippetChars-3] + "..."
	}
	if len(lines) > len(snippetLines) {
		if snippet != "" {
			snippet += "\n..."
		} else {
			snippet = "..."
		}
	}
	return HunkEvidence{Header: compactHeader(header), Snippet: snippet, Score: score}
}

func compactHeader(header string) string {
	value := strings.TrimSpace(header)
	if len(value) > 180 {
		return value[:177] + "..."
	}
	return value
}

func scoreHeader(header string) int {
	value := strings.ToLower(header)
	score := 10
	keywords := []string{"func ", "type ", "method", "class ", "interface", "struct", "handler", "route", "command", "query", "migration", "schema", "table"}
	for _, keyword := range keywords {
		if strings.Contains(value, keyword) {
			score += 18
		}
	}
	return score
}

func scoreChangedLine(line string) int {
	value := strings.ToLower(line)
	score := 4
	keywords := []string{"func", "type", "struct", "interface", "return", "if ", "switch", "case ", "select", "insert", "update", "delete", "create", "drop", "alter", "validate", "error", "commit", "prompt", "message", "scope", "token"}
	for _, keyword := range keywords {
		if strings.Contains(value, keyword) {
			score += 8
		}
	}
	if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
		score += 2
	}
	if len(strings.TrimSpace(strings.TrimLeft(line, "+-"))) == 0 {
		score -= 2
	}
	return score
}

func scoreFile(file FileEvidence) int {
	score := file.Additions + file.Deletions
	switch file.Status {
	case "A", "D":
		score += 28
	case "R", "C":
		score += 24
	default:
		score += 12
	}
	switch file.Kind {
	case "source":
		score += 48
	case "migration":
		score += 42
	case "config":
		score += 28
	case "build":
		score += 24
	case "test":
		score += 14
	case "docs":
		score += 8
	case "meta":
		score += 6
	case "lockfile":
		score -= 18
	case "generated":
		score -= 36
	case "binary":
		score -= 60
	}
	for _, hunk := range file.Hunks {
		score += hunk.Score / 4
	}
	if score < 0 {
		return 0
	}
	return score
}

func classifyPath(path string) string {
	normalized := strings.ToLower(normalizePath(path))
	base := strings.ToLower(filepath.Base(normalized))
	ext := strings.ToLower(filepath.Ext(base))

	if normalized == "go.sum" || normalized == "package-lock.json" || normalized == "yarn.lock" || normalized == "pnpm-lock.yaml" || normalized == "cargo.lock" {
		return "lockfile"
	}
	if base == "dockerfile" || base == "makefile" || strings.HasPrefix(normalized, ".github/workflows/") {
		return "build"
	}
	if strings.Contains(normalized, "/migrations/") || strings.Contains(normalized, "/migration/") || ext == ".sql" {
		return "migration"
	}
	if strings.Contains(base, "_test.") || strings.Contains(base, ".spec.") || strings.Contains(base, ".test.") || strings.Contains(normalized, "/test/") || strings.Contains(normalized, "/tests/") {
		return "test"
	}
	if ext == ".md" || ext == ".rst" || ext == ".txt" || ext == ".adoc" || strings.HasPrefix(normalized, "docs/") {
		return "docs"
	}
	if base == ".gitignore" || base == ".editorconfig" || base == "license" || base == "go.mod" {
		return "meta"
	}
	if ext == ".json" || ext == ".yaml" || ext == ".yml" || ext == ".toml" || ext == ".ini" || ext == ".conf" || ext == ".cfg" || ext == ".env" || ext == ".xml" || strings.Contains(normalized, "/config/") {
		return "config"
	}
	if strings.Contains(normalized, "/vendor/") || strings.Contains(normalized, "/dist/") || strings.Contains(normalized, "/build/") || strings.HasSuffix(base, ".pb.go") || strings.HasSuffix(base, ".generated.go") || strings.HasSuffix(base, ".min.js") {
		return "generated"
	}
	if isBinaryExtension(ext) {
		return "binary"
	}
	if isSourceExtension(ext) || base == "docker-compose" {
		return "source"
	}
	return "meta"
}

func isSourceExtension(ext string) bool {
	switch ext {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".rs", ".java", ".kt", ".swift", ".c", ".cc", ".cpp", ".h", ".hpp", ".cs", ".php", ".scala", ".sh", ".bash", ".zsh":
		return true
	default:
		return false
	}
}

func isBinaryExtension(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".pdf", ".zip", ".tar", ".gz", ".tgz", ".jar", ".exe", ".dll", ".so", ".dylib", ".bin":
		return true
	default:
		return false
	}
}

func parseDirStat(raw string) []ModuleWeight {
	var modules []ModuleWeight
	for _, line := range splitLines(raw) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		percent, err := strconv.ParseFloat(strings.TrimSuffix(fields[0], "%"), 64)
		if err != nil {
			continue
		}
		path := normalizePath(fields[1])
		if path == "" {
			continue
		}
		modules = append(modules, ModuleWeight{Path: path, Percent: percent})
	}
	sort.SliceStable(modules, func(i, j int) bool {
		if modules[i].Percent == modules[j].Percent {
			return modules[i].Path < modules[j].Path
		}
		return modules[i].Percent > modules[j].Percent
	})
	return modules
}

func deriveModules(files []FileEvidence) []ModuleWeight {
	weights := make(map[string]float64)
	var total float64
	for _, file := range files {
		module := dominantModuleFromPath(file.Path)
		if module == "" {
			continue
		}
		weight := float64(file.Additions + file.Deletions)
		if weight == 0 {
			weight = 1
		}
		weights[module] += weight
		total += weight
	}
	if total == 0 {
		return nil
	}
	modules := make([]ModuleWeight, 0, len(weights))
	for path, weight := range weights {
		modules = append(modules, ModuleWeight{Path: path, Percent: weight * 100 / total})
	}
	sort.SliceStable(modules, func(i, j int) bool {
		if modules[i].Percent == modules[j].Percent {
			return modules[i].Path < modules[j].Path
		}
		return modules[i].Percent > modules[j].Percent
	})
	return modules
}

func dominantModuleFromPath(path string) string {
	normalized := normalizePath(path)
	parts := strings.Split(normalized, "/")
	if len(parts) == 0 {
		return ""
	}
	switch parts[0] {
	case "internal", "cmd", "pkg", "src", "lib", "app":
		if len(parts) > 1 {
			return parts[0] + "/" + parts[1]
		}
		return parts[0]
	case ".github":
		if len(parts) > 1 {
			return parts[0] + "/" + parts[1]
		}
		return ".github"
	default:
		if len(parts) > 1 {
			return parts[0]
		}
		return strings.TrimSuffix(parts[0], filepath.Ext(parts[0]))
	}
}

func scopeCandidate(path string) string {
	normalized := normalizePath(path)
	if normalized == "" {
		return ""
	}
	parts := strings.Split(normalized, "/")
	if len(parts) == 0 {
		return ""
	}
	switch parts[0] {
	case "internal", "cmd", "pkg", "src", "app", "lib":
		if len(parts) > 1 {
			return sanitizeScope(parts[1])
		}
		return sanitizeScope(parts[0])
	case ".github":
		if len(parts) > 1 && parts[1] == "workflows" {
			return "ci"
		}
		return "github"
	case "docs":
		return "docs"
	default:
		if len(parts) > 1 {
			return sanitizeScope(parts[0])
		}
		base := strings.TrimSuffix(filepath.Base(normalized), filepath.Ext(normalized))
		return sanitizeScope(base)
	}
}

func sanitizeScope(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Trim(value, ".-_ ")
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(" ", "-", "_", "-", "/", "-", "\\", "-")
	return replacer.Replace(value)
}

func normalizeOverview(overview EvidenceOverview) EvidenceOverview {
	if overview.FileCount < 0 {
		overview.FileCount = 0
	}
	return overview
}

func estimateLineChanges(hunks []HunkEvidence) (int, int) {
	var additions int
	var deletions int
	for _, hunk := range hunks {
		for _, line := range strings.Split(hunk.Snippet, "\n") {
			switch {
			case strings.HasPrefix(line, "+"):
				additions++
			case strings.HasPrefix(line, "-"):
				deletions++
			}
		}
	}
	return additions, deletions
}

func firstStatusCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1])
}

func normalizeGitDiffPath(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "a/")
	value = strings.TrimPrefix(value, "b/")
	if value == "/dev/null" {
		return ""
	}
	return normalizePath(value)
}

func normalizePath(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.TrimPrefix(value, "./")
	value = strings.Trim(value, "\"'")
	return value
}

func normalizeStatPath(value string) string {
	value = normalizePath(value)
	if strings.Contains(value, "=>") {
		if strings.Contains(value, "{") && strings.Contains(value, "}") {
			start := strings.Index(value, "{")
			end := strings.Index(value, "}")
			if start >= 0 && end > start {
				prefix := value[:start]
				body := value[start+1 : end]
				suffix := value[end+1:]
				parts := strings.SplitN(body, "=>", 2)
				if len(parts) == 2 {
					return normalizePath(prefix + strings.TrimSpace(parts[1]) + suffix)
				}
			}
		}
		parts := strings.Split(value, "=>")
		return normalizePath(parts[len(parts)-1])
	}
	return value
}

func parseNumStatValue(value string) (int, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "-" {
		return 0, false
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func splitLines(raw string) []string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimRight(part, "\r")
		if strings.TrimSpace(part) != "" {
			out = append(out, part)
		}
	}
	return out
}

func trimChangedLine(line string) string {
	if line == "" {
		return ""
	}
	prefix := line[:1]
	content := strings.TrimSpace(line[1:])
	if content == "" {
		return prefix
	}
	if len(content) > 140 {
		content = content[:137] + "..."
	}
	return prefix + content
}

func pluralize(count int, singular string, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}
