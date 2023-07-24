package termkit

import (
	"fmt"
	"testing"
)

func TestPrintln(t *testing.T) {
	FgHiRed.Println("hiRed")
	fmt.Println("normal")
	FgHiCyan.Println("hiCyan")
	fmt.Println("normal")
}

func TestPrintf(t *testing.T) {
	FgHiGreen.Printf("%s", "hiGreen")
	fmt.Println("normal")
}

func TestRender(t *testing.T) {
	green := FgGreen.Render
	hiGreen := FgHiGreen.Render
	blue := FgBlue.Render
	magenta := FgHiMagenta.Render
	fmt.Println("normal", blue("blue"), magenta("magenta"), green("green"), hiGreen("hiGreen"))
	fmt.Println("normal")
}
