package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHostCommandHelpAndVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runCommand([]string{"help"}, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("help exit = %d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "host_speak") {
		t.Fatalf("help output = %s", out.String())
	}

	out.Reset()
	if code := runCommand([]string{"version"}, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("version exit = %d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "tars-stackchan-host") {
		t.Fatalf("version output = %s", out.String())
	}
}
