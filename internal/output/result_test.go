package output

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
	"time"
)

type sampleData struct {
	Name string `json:"name"`
}

func TestEmit_SuccessJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := Emit(&stdout, &stderr, "1.0.0", time.Now(), RenderOptions{JSONMode: true},
		Result{Data: sampleData{Name: "watch-a"}}, nil)

	if exit != ExitOK {
		t.Fatalf("exit = %d, want %d", exit, ExitOK)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty in JSON mode, got %q", stderr.String())
	}

	var env Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not valid JSON: %v (%s)", err, stdout.String())
	}
	if !env.OK || env.Error != nil {
		t.Fatalf("expected ok envelope, got %+v", env)
	}
}

func TestEmit_FailureJSON_StdoutStillValidEnvelope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmdErr := NewError(CodeNotFound, "地点不存在").WithHint("换个关键词试试").WithNextCommands("bluebird place search foo")

	exit := Emit(&stdout, &stderr, "1.0.0", time.Now(), RenderOptions{JSONMode: true},
		Result{Err: cmdErr}, nil)

	if exit != ExitNotFound {
		t.Fatalf("exit = %d, want %d", exit, ExitNotFound)
	}

	var env Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("失败路径 stdout 必须仍是合法 JSON envelope: %v (%s)", err, stdout.String())
	}
	if env.OK {
		t.Fatal("expected ok=false")
	}
	if env.Data != nil {
		t.Fatalf("expected data=nil on failure, got %v", env.Data)
	}
	if env.Error == nil || env.Error.Code != CodeNotFound {
		t.Fatalf("expected error.code=%s, got %+v", CodeNotFound, env.Error)
	}
}

func TestEmit_FailureHuman_WritesStderrNotStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmdErr := NewError(CodeUnauthorized, "鉴权失败")

	exit := Emit(&stdout, &stderr, "1.0.0", time.Now(), RenderOptions{JSONMode: false},
		Result{Err: cmdErr}, nil)

	if exit != ExitAuth {
		t.Fatalf("exit = %d, want %d", exit, ExitAuth)
	}
	if stdout.Len() != 0 {
		t.Fatalf("人类模式失败不应向 stdout 写任何内容，got %q", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("人类模式失败应向 stderr 写中文错误文本")
	}
}

func TestEmit_HumanSuccess_UsesRenderFunc(t *testing.T) {
	var stdout, stderr bytes.Buffer
	called := false
	human := func(w io.Writer, data any, quiet bool) error {
		called = true
		_, err := w.Write([]byte("rendered\n"))
		return err
	}

	exit := Emit(&stdout, &stderr, "1.0.0", time.Now(), RenderOptions{JSONMode: false},
		Result{Data: sampleData{Name: "x"}}, human)

	if exit != ExitOK {
		t.Fatalf("exit = %d, want %d", exit, ExitOK)
	}
	if !called {
		t.Fatal("human render func was not called")
	}
	if stdout.String() != "rendered\n" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "rendered\n")
	}
}
