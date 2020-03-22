package conf

import (
	"flag"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	type dummy struct {
		fromDefault  string
		fromCLI      string
		fromConf     string
		priorityCLI  string
		priorityConf string
		quotedConf   string

		nakedBool       bool
		normalBool      bool
		normalFalseBool bool
		quotedBool      bool
	}

	got := dummy{
		normalFalseBool: true,
	}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.StringVar(&got.fromDefault, "from-default", "value-default", "value taken from default")
	fs.StringVar(&got.fromCLI, "from-cli", "value-default", "value taken from CLI")
	fs.StringVar(&got.fromConf, "from-conf", "value-default", "value taken from conf")
	fs.StringVar(&got.priorityCLI, "priority-cli", "value-default", "value taken from CLI over others")
	fs.StringVar(&got.priorityConf, "priority-conf", "value-default", "value taken from conf over default")
	fs.StringVar(&got.quotedConf, "quoted-conf", "value-default", "value taken from conf and unquoted")
	fs.BoolVar(&got.nakedBool, "naked-bool", false, "value taken from a naked flag in conf")
	fs.BoolVar(&got.normalBool, "normal-bool", false, "value taken from conf")
	fs.BoolVar(&got.normalFalseBool, "normal-false-bool", true, "value taken from conf")
	fs.BoolVar(&got.quotedBool, "quoted-bool", false, "value taken from conf and unquoted")
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
		fromDefault:     "value-default",
		fromCLI:         "value-cli",
		fromConf:        "value-conf",
		priorityCLI:     "value-cli",
		priorityConf:    "value-conf",
		quotedConf:      "value-conf",
		nakedBool:       true,
		normalBool:      true,
		normalFalseBool: false,
		quotedBool:      true,
	}

	if !reflect.DeepEqual(expected, got) {
		t.Errorf("incorrect result:\nwanted %#v,\n   got %#v", expected, got)
	}
}
