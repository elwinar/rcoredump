package conf

import (
	"flag"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParse(t *testing.T) {
	type dummy struct {
		FromDefault  string
		FromCLI      string
		FromConf     string
		PriorityCLI  string
		PriorityConf string
		QuotedConf   string

		NakedBool       bool
		NormalBool      bool
		NormalFalseBool bool
		QuotedBool      bool
	}

	got := dummy{
		NormalFalseBool: true,
	}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.StringVar(&got.FromDefault, "from-default", "value-default", "value taken from default")
	fs.StringVar(&got.FromCLI, "from-cli", "value-default", "value taken from CLI")
	fs.StringVar(&got.FromConf, "from-conf", "value-default", "value taken from conf")
	fs.StringVar(&got.PriorityCLI, "priority-cli", "value-default", "value taken from CLI over others")
	fs.StringVar(&got.PriorityConf, "priority-conf", "value-default", "value taken from conf over default")
	fs.StringVar(&got.QuotedConf, "quoted-conf", "value-default", "value taken from conf and unquoted")
	fs.BoolVar(&got.NakedBool, "naked-bool", false, "value taken from a naked flag in conf")
	fs.BoolVar(&got.NormalBool, "normal-bool", false, "value taken from conf")
	fs.BoolVar(&got.NormalFalseBool, "normal-false-bool", true, "value taken from conf")
	fs.BoolVar(&got.QuotedBool, "quoted-bool", false, "value taken from conf and unquoted")
	fs.String("c", "./testdata/test.conf", "configuration file path")

	args := []string{
		"-from-cli", "value-cli",
		"-priority-cli", "value-cli",
	}

	err := parse(fs, args, "c")
	if err != nil {
		t.Errorf(`unexpected error: got %#v`, err)
	}

	expected := dummy{
		FromDefault:     "value-default",
		FromCLI:         "value-cli",
		FromConf:        "value-conf",
		PriorityCLI:     "value-cli",
		PriorityConf:    "value-conf",
		QuotedConf:      "value-conf",
		NakedBool:       true,
		NormalBool:      true,
		NormalFalseBool: false,
		QuotedBool:      true,
	}

	if !cmp.Equal(expected, got) {
		t.Errorf(`Parse(): unexpected result`)
		t.Log(cmp.Diff(expected, got))
	}
}

func TestMapFlag(t *testing.T) {
	type dummy struct {
		m map[string]string
	}

	got := dummy{}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(MapFlag(&got.m), "m", "map value")

	args := []string{
		"-m", "key-1=value-1",
		"-m", "key-2=value-2",
		"-m", "key-3=value-3;key-4=value-4",
	}

	fs.Parse(args)

	expected := dummy{
		m: map[string]string{
			"key-1": "value-1",
			"key-2": "value-2",
			"key-3": "value-3",
			"key-4": "value-4",
		},
	}

	if !reflect.DeepEqual(expected, got) {
		t.Errorf("incorrect result:\nwanted %#v,\n   got %#v", expected, got)
	}
}
