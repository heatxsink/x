package dotenv

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

const defaultFilename = ".env"

var exportPrefix = regexp.MustCompile(`^\s*export\s+`)

// Load reads the given .env files and sets the values in os.Environ.
// It does NOT override existing environment variables.
// If no filenames are given, it loads ".env".
func Load(filenames ...string) error {
	filenames = defaultFilenames(filenames)
	for _, f := range filenames {
		envMap, err := readFile(f)
		if err != nil {
			return err
		}
		for k, v := range envMap {
			if _, ok := os.LookupEnv(k); !ok {
				os.Setenv(k, v)
			}
		}
	}
	return nil
}

// Overload reads the given .env files and sets the values in os.Environ.
// It DOES override existing environment variables.
// If no filenames are given, it loads ".env".
func Overload(filenames ...string) error {
	filenames = defaultFilenames(filenames)
	for _, f := range filenames {
		envMap, err := readFile(f)
		if err != nil {
			return err
		}
		for k, v := range envMap {
			os.Setenv(k, v)
		}
	}
	return nil
}

// Read reads the given .env files and returns a merged map of key-value pairs.
// Later files override earlier files. Does not modify os.Environ.
// If no filenames are given, it reads ".env".
func Read(filenames ...string) (map[string]string, error) {
	filenames = defaultFilenames(filenames)
	merged := map[string]string{}
	for _, f := range filenames {
		envMap, err := readFile(f)
		if err != nil {
			return nil, err
		}
		for k, v := range envMap {
			merged[k] = v
		}
	}
	return merged, nil
}

// Parse reads an io.Reader and returns a map of key-value pairs.
func Parse(r io.Reader) (map[string]string, error) {
	return parseReader(r)
}

// Unmarshal parses a string in .env format and returns a map of key-value pairs.
func Unmarshal(str string) (map[string]string, error) {
	return parseReader(strings.NewReader(str))
}

// UnmarshalBytes parses a byte slice in .env format and returns a map of key-value pairs.
func UnmarshalBytes(src []byte) (map[string]string, error) {
	return parseReader(strings.NewReader(string(src)))
}

// Marshal converts a map of key-value pairs to .env format.
// Keys are sorted alphabetically. Values are double-quoted with special
// characters escaped.
func Marshal(envMap map[string]string) (string, error) {
	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		v := envMap[k]
		fmt.Fprintf(&b, "%s=%q\n", k, v)
	}
	return b.String(), nil
}

// Write marshals the given map and writes it to a file.
func Write(envMap map[string]string, filename string) error {
	content, err := Marshal(envMap)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, []byte(content), 0644)
}

// Exec loads the given .env files into the environment and executes the
// command. If overload is true, existing variables are overridden.
func Exec(filenames []string, cmd string, cmdArgs []string, overload bool) error {
	if overload {
		if err := Overload(filenames...); err != nil {
			return err
		}
	} else {
		if err := Load(filenames...); err != nil {
			return err
		}
	}
	command := exec.Command(cmd, cmdArgs...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = os.Environ()
	return command.Run()
}

func defaultFilenames(filenames []string) []string {
	if len(filenames) == 0 {
		return []string{defaultFilename}
	}
	return filenames
}

func readFile(filename string) (map[string]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseReader(f)
}

func parseReader(r io.Reader) (map[string]string, error) {
	envMap := map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.Replace(line, "\r", "", -1)
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		line = exportPrefix.ReplaceAllString(line, "")
		key, value, err := parseLine(line, envMap)
		if err != nil {
			return nil, err
		}
		envMap[key] = value
	}
	return envMap, scanner.Err()
}

func parseLine(line string, envMap map[string]string) (string, string, error) {
	sep := strings.IndexAny(line, "=:")
	if sep < 0 {
		return "", "", fmt.Errorf("dotenv: invalid line: %s", line)
	}
	key := strings.TrimSpace(line[:sep])
	value := strings.TrimSpace(line[sep+1:])
	if len(value) == 0 {
		return key, "", nil
	}
	switch value[0] {
	case '\'':
		value = parseSingleQuoted(value)
	case '"':
		value = parseDoubleQuoted(value, envMap)
	case '`':
		value = parseBacktickQuoted(value)
	default:
		value = parseUnquoted(value, envMap)
	}
	return key, value, nil
}

func parseSingleQuoted(value string) string {
	end := strings.LastIndex(value, "'")
	if end <= 0 {
		return value[1:]
	}
	return value[1:end]
}

const escapedDollarPlaceholder = "\x00ESCAPED_DOLLAR\x00"

func parseDoubleQuoted(value string, envMap map[string]string) string {
	end := strings.LastIndex(value, "\"")
	if end <= 0 {
		return value[1:]
	}
	inner := value[1:end]
	// Replace escaped dollars before variable expansion to preserve them.
	inner = strings.ReplaceAll(inner, `\$`, escapedDollarPlaceholder)
	inner = expandEscapes(inner)
	inner = expandVariables(inner, envMap)
	return strings.ReplaceAll(inner, escapedDollarPlaceholder, "$")
}

func parseBacktickQuoted(value string) string {
	end := strings.LastIndex(value, "`")
	if end <= 0 {
		return value[1:]
	}
	return value[1:end]
}

func parseUnquoted(value string, envMap map[string]string) string {
	if idx := strings.Index(value, " #"); idx >= 0 {
		value = value[:idx]
	}
	value = strings.TrimSpace(value)
	return expandVariables(value, envMap)
}

func expandEscapes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			case '`':
				b.WriteByte('`')
			default:
				b.WriteByte('\\')
				b.WriteByte(s[i+1])
			}
			i++
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

var varPattern = regexp.MustCompile(`\$\{([A-Za-z0-9_.]+)\}|\$([A-Za-z0-9_.]+)`)

func expandVariables(s string, envMap map[string]string) string {
	return varPattern.ReplaceAllStringFunc(s, func(match string) string {
		var name string
		if strings.HasPrefix(match, "${") {
			name = match[2 : len(match)-1]
		} else {
			name = match[1:]
		}
		if v, ok := envMap[name]; ok {
			return v
		}
		if v, ok := os.LookupEnv(name); ok {
			return v
		}
		return ""
	})
}
