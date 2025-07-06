package shell

import (
	"os"
	"testing"
)

func TestExecute(t *testing.T) {
	err := Execute("echo", "test")
	if err != nil {
		t.Errorf("Execute should succeed with valid command: %v", err)
	}
}

func TestExecuteWithArgs(t *testing.T) {
	err := Execute("echo", "hello", "world")
	if err != nil {
		t.Errorf("Execute should succeed with multiple args: %v", err)
	}
}

func TestExecuteWithPath(t *testing.T) {
	err := Execute("ls", "/dev/null")
	if err != nil {
		t.Errorf("Execute should succeed with path argument: %v", err)
	}
}

func TestExecuteFail(t *testing.T) {
	err := Execute("nonexistent-command-xyz")
	if err == nil {
		t.Error("Execute should fail with nonexistent command")
	}
}

func TestExecuteFailWithArgs(t *testing.T) {
	err := Execute("nonexistent-command", "arg1", "arg2")
	if err == nil {
		t.Error("Execute should fail with nonexistent command and args")
	}
}

func TestExecuteWith(t *testing.T) {
	env := map[string]string{
		"TEST_VAR": "test_value",
	}
	ExecuteWith(env, "sh", "-c", "echo $TEST_VAR")
}

func TestExecuteWithMultipleEnvVars(t *testing.T) {
	env := map[string]string{
		"VAR1": "value1",
		"VAR2": "value2",
		"VAR3": "value3",
	}
	ExecuteWith(env, "sh", "-c", "echo $VAR1 $VAR2 $VAR3")
}

func TestExecuteWithEmptyEnv(t *testing.T) {
	env := map[string]string{}
	ExecuteWith(env, "echo", "test")
}

func TestExecuteWithNilEnv(t *testing.T) {
	ExecuteWith(nil, "echo", "test")
}

func TestExecuteWithExistingEnvOverride(t *testing.T) {
	// Set an environment variable
	os.Setenv("SHELL_TEST_VAR", "original")
	defer os.Unsetenv("SHELL_TEST_VAR")

	// Override it in our execution
	env := map[string]string{
		"SHELL_TEST_VAR": "overridden",
	}
	ExecuteWith(env, "sh", "-c", "echo $SHELL_TEST_VAR")
}

func TestExecuteWithInvalidCommand(t *testing.T) {
	env := map[string]string{
		"TEST_VAR": "test",
	}
	ExecuteWith(env, "invalid-command-that-does-not-exist")
}

func TestExecuteScriptCommand(t *testing.T) {
	err := Execute("sh", "-c", "echo 'Hello from script' && exit 0")
	if err != nil {
		t.Errorf("Execute should succeed with shell script: %v", err)
	}
}

func TestExecuteFailingScriptCommand(t *testing.T) {
	err := Execute("sh", "-c", "echo 'This will fail' && exit 1")
	if err == nil {
		t.Error("Execute should fail when script exits with non-zero status")
	}
}

func TestExecuteWithLongOutput(t *testing.T) {
	// Test with command that produces significant output
	err := Execute("sh", "-c", "for i in {1..10}; do echo \"Line $i\"; done")
	if err != nil {
		t.Errorf("Execute should handle long output: %v", err)
	}
}

func TestExecuteWithQuotedArgs(t *testing.T) {
	err := Execute("echo", "hello world", "test with spaces")
	if err != nil {
		t.Errorf("Execute should handle quoted arguments: %v", err)
	}
}

func TestExecuteWithSpecialCharacters(t *testing.T) {
	err := Execute("echo", "special chars: !@#$%^&*()")
	if err != nil {
		t.Errorf("Execute should handle special characters: %v", err)
	}
}
