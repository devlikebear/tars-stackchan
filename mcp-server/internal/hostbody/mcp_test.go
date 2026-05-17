package hostbody

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestHostMCPToolsExposeSpeechOnlyActions(t *testing.T) {
	tools := ListTools(Tools{Say: "/usr/bin/say"})
	if len(tools) != 2 {
		t.Fatalf("tools = %+v", tools)
	}
	if tools[0].Name != ToolHostGetStatus || tools[1].Name != ToolHostSpeak {
		t.Fatalf("tools = %+v", tools)
	}
}

func TestCallToolRoutesHostSpeak(t *testing.T) {
	runner := &recordingRunner{}
	host := Host{
		Tools:   Tools{Say: "/usr/bin/say"},
		Runner:  runner,
		WorkDir: t.TempDir(),
	}
	raw := json.RawMessage(`{"text":"hello from TARS"}`)
	result, err := CallTool(context.Background(), host, ToolHostSpeak, raw)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if runner.command != "/usr/bin/say" || !containsArg(runner.args, "hello from TARS") {
		t.Fatalf("runner call = %s %v", runner.command, runner.args)
	}
	if len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, `"ok":true`) {
		t.Fatalf("result = %+v", result)
	}
}
