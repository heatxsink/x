package dotenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSimple(t *testing.T) {
	input := "FOO=bar\nBAZ=qux\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", m["FOO"], "bar")
	}
	if m["BAZ"] != "qux" {
		t.Errorf("BAZ = %q, want %q", m["BAZ"], "qux")
	}
}

func TestParseComments(t *testing.T) {
	input := "# this is a comment\nFOO=bar\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 {
		t.Errorf("expected 1 entry, got %d", len(m))
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", m["FOO"], "bar")
	}
}

func TestParseInlineComments(t *testing.T) {
	input := "FOO=bar # this is a comment\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", m["FOO"], "bar")
	}
}

func TestParseSingleQuotes(t *testing.T) {
	input := "FOO='bar baz'\nNOEXPAND='$HOME'\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar baz" {
		t.Errorf("FOO = %q, want %q", m["FOO"], "bar baz")
	}
	if m["NOEXPAND"] != "$HOME" {
		t.Errorf("NOEXPAND = %q, want %q", m["NOEXPAND"], "$HOME")
	}
}

func TestParseDoubleQuotes(t *testing.T) {
	input := `FOO="bar baz"` + "\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar baz" {
		t.Errorf("FOO = %q, want %q", m["FOO"], "bar baz")
	}
}

func TestParseDoubleQuoteEscapes(t *testing.T) {
	input := `FOO="line1\nline2"` + "\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "line1\nline2" {
		t.Errorf("FOO = %q, want %q", m["FOO"], "line1\nline2")
	}
}

func TestParseBacktickQuotes(t *testing.T) {
	input := "FOO=`bar baz`\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar baz" {
		t.Errorf("FOO = %q, want %q", m["FOO"], "bar baz")
	}
}

func TestParseExportPrefix(t *testing.T) {
	input := "export FOO=bar\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", m["FOO"], "bar")
	}
}

func TestParseColonDelimiter(t *testing.T) {
	input := "FOO: bar\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", m["FOO"], "bar")
	}
}

func TestParseEmptyValue(t *testing.T) {
	input := "FOO=\nBAR=''\nBAZ=\"\"\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"FOO", "BAR", "BAZ"} {
		if m[k] != "" {
			t.Errorf("%s = %q, want empty", k, m[k])
		}
	}
}

func TestParseVariableExpansion(t *testing.T) {
	input := "BASE=/usr/local\nPATH_A=\"$BASE/bin\"\nPATH_B=\"${BASE}/lib\"\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["PATH_A"] != "/usr/local/bin" {
		t.Errorf("PATH_A = %q, want %q", m["PATH_A"], "/usr/local/bin")
	}
	if m["PATH_B"] != "/usr/local/lib" {
		t.Errorf("PATH_B = %q, want %q", m["PATH_B"], "/usr/local/lib")
	}
}

func TestParseNoExpansionInSingleQuotes(t *testing.T) {
	input := "BASE=/usr/local\nPATH_A='$BASE/bin'\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["PATH_A"] != "$BASE/bin" {
		t.Errorf("PATH_A = %q, want %q", m["PATH_A"], "$BASE/bin")
	}
}

func TestParseWhitespaceAroundDelimiter(t *testing.T) {
	input := "FOO = bar\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", m["FOO"], "bar")
	}
}

func TestParseCRLF(t *testing.T) {
	input := "FOO=bar\r\nBAZ=qux\r\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" || m["BAZ"] != "qux" {
		t.Errorf("CRLF parsing failed: %v", m)
	}
}

func TestParseBlankLines(t *testing.T) {
	input := "\n\nFOO=bar\n\n\nBAZ=qux\n\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Errorf("expected 2 entries, got %d", len(m))
	}
}

func TestUnmarshalBytes(t *testing.T) {
	input := []byte("FOO=bar\n")
	m, err := UnmarshalBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", m["FOO"], "bar")
	}
}

func TestParseFromReader(t *testing.T) {
	r := strings.NewReader("FOO=bar\n")
	m, err := Parse(r)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", m["FOO"], "bar")
	}
}

func TestMarshal(t *testing.T) {
	envMap := map[string]string{
		"B_KEY": "value2",
		"A_KEY": "value1",
	}
	s, err := Marshal(envMap)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "A_KEY=") {
		t.Errorf("expected sorted output, first line: %s", lines[0])
	}
}

func TestMarshalEscaping(t *testing.T) {
	envMap := map[string]string{
		"FOO": "line1\nline2",
	}
	s, err := Marshal(envMap)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, `\n`) {
		t.Errorf("expected escaped newline in: %s", s)
	}
}

func TestLoadDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("TEST_DOTENV_NO_OVERRIDE=from_file\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TEST_DOTENV_NO_OVERRIDE", "from_env")

	err := Load(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("TEST_DOTENV_NO_OVERRIDE") != "from_env" {
		t.Error("Load should not override existing env vars")
	}
}

func TestOverloadDoesOverride(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("TEST_DOTENV_OVERRIDE=from_file\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TEST_DOTENV_OVERRIDE", "from_env")

	err := Overload(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("TEST_DOTENV_OVERRIDE") != "from_file" {
		t.Error("Overload should override existing env vars")
	}
}

func TestReadDoesNotModifyEnv(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("TEST_DOTENV_READ_ONLY=secret\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := Read(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if m["TEST_DOTENV_READ_ONLY"] != "secret" {
		t.Errorf("expected 'secret', got %q", m["TEST_DOTENV_READ_ONLY"])
	}
	if os.Getenv("TEST_DOTENV_READ_ONLY") != "" {
		t.Error("Read should not modify os.Environ")
	}
}

func TestWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "test.env")
	original := map[string]string{
		"FOO": "bar",
		"BAZ": "line1\nline2",
	}
	err := Write(original, envFile)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Read(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", m["FOO"], "bar")
	}
	if m["BAZ"] != "line1\nline2" {
		t.Errorf("BAZ = %q, want %q", m["BAZ"], "line1\nline2")
	}
}

func TestLoadDefaultFilename(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("TEST_DOTENV_DEFAULT=yes\n"), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("TEST_DOTENV_DEFAULT") != "yes" {
		t.Error("Load with no args should read .env")
	}
	os.Unsetenv("TEST_DOTENV_DEFAULT")
}

func TestVariableExpansionFromEnv(t *testing.T) {
	t.Setenv("EXISTING_VAR", "from_env")
	input := `EXPANDED="$EXISTING_VAR/path"` + "\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["EXPANDED"] != "from_env/path" {
		t.Errorf("EXPANDED = %q, want %q", m["EXPANDED"], "from_env/path")
	}
}

func TestParseEscapedDollar(t *testing.T) {
	input := `FOO="price is \$100"` + "\n"
	m, err := Unmarshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "price is $100" {
		t.Errorf("FOO = %q, want %q", m["FOO"], "price is $100")
	}
}
