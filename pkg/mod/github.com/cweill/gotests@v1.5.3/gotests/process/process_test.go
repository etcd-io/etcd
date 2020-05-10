package process

import (
	"bytes"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name string
		args []string
		opts *Options
		want string
	}{
		// TODO: Add test cases.
		{
			name: "Nil options and nil args",
			args: nil,
			opts: nil,
			want: specifyFlagMessage + "\n",
		}, {
			name: "Nil options",
			args: []string{"testdata/foobar.go"},
			opts: nil,
			want: specifyFlagMessage + "\n",
		}, {
			name: "Empty options",
			args: []string{"testdata/foobar.go"},
			opts: &Options{},
			want: specifyFlagMessage + "\n",
		}, {
			name: "Non-empty options with no args",
			args: []string{},
			opts: &Options{AllFuncs: true},
			want: specifyFileMessage + "\n",
		}, {
			name: "OnlyFuncs option w/ no matches",
			args: []string{"testdata/foobar.go"},
			opts: &Options{OnlyFuncs: "FooBar"},
			want: "No tests generated for testdata/foobar.go\n",
		}, {
			name: "Invalid OnlyFuncs option",
			args: []string{"testdata/foobar.go"},
			opts: &Options{OnlyFuncs: "??"},
			want: "Invalid -only regex: error parsing regexp: missing argument to repetition operator: `??`\n",
		}, {
			name: "Invalid ExclFuncs option",
			args: []string{"testdata/foobar.go"},
			opts: &Options{ExclFuncs: "??"},
			want: "Invalid -excl regex: error parsing regexp: missing argument to repetition operator: `??`\n",
		},
	}
	for _, tt := range tests {
		out := &bytes.Buffer{}
		Run(out, tt.args, tt.opts)
		if got := out.String(); got != tt.want {
			t.Errorf("%q. Run() =\n%v, want\n%v", tt.name, got, tt.want)
		}
	}
}
