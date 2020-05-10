package main

import (
	"reflect"
	"testing"

	"github.com/josharian/impl/testdata"
)

type errBool bool

func (b errBool) String() string {
	if b {
		return "an error"
	}
	return "no error"
}

func TestFindInterface(t *testing.T) {
	cases := []struct {
		iface   string
		path    string
		id      string
		wantErr bool
	}{
		{iface: "net.Conn", path: "net", id: "Conn"},
		{iface: "http.ResponseWriter", path: "net/http", id: "ResponseWriter"},
		{iface: "net.Tennis", wantErr: true},
		{iface: "a + b", wantErr: true},
		{iface: "a/b/c/", wantErr: true},
		{iface: "a/b/c/pkg", wantErr: true},
		{iface: "a/b/c/pkg.", wantErr: true},
		{iface: "a/b/c/pkg.Typ", path: "a/b/c/pkg", id: "Typ"},
		{iface: "a/b/c/pkg.Typ.Foo", wantErr: true},
	}

	for _, tt := range cases {
		path, id, err := findInterface(tt.iface, ".")
		gotErr := err != nil
		if tt.wantErr != gotErr {
			t.Errorf("findInterface(%q).err=%v want %s", tt.iface, err, errBool(tt.wantErr))
			continue
		}
		if tt.path != path {
			t.Errorf("findInterface(%q).path=%q want %q", tt.iface, path, tt.path)
		}
		if tt.id != id {
			t.Errorf("findInterface(%q).id=%q want %q", tt.iface, id, tt.id)
		}
	}
}

func TestTypeSpec(t *testing.T) {
	// For now, just test whether we can find the interface.
	cases := []struct {
		path    string
		id      string
		wantErr bool
	}{
		{path: "net", id: "Conn"},
		{path: "net", id: "Con", wantErr: true},
	}

	for _, tt := range cases {
		p, spec, err := typeSpec(tt.path, tt.id, "")
		gotErr := err != nil
		if tt.wantErr != gotErr {
			t.Errorf("typeSpec(%q, %q).err=%v want %s", tt.path, tt.id, err, errBool(tt.wantErr))
			continue
		}
		if err == nil {
			if reflect.DeepEqual(p, Pkg{}) {
				t.Errorf("typeSpec(%q, %q).pkg=Pkg{} want non-nil", tt.path, tt.id)
			}
			if reflect.DeepEqual(spec, Spec{}) {
				t.Errorf("typeSpec(%q, %q).spec=Spec{} want non-nil", tt.path, tt.id)
			}
		}
	}
}

func TestFuncs(t *testing.T) {
	cases := []struct {
		iface   string
		want    []Func
		wantErr bool
	}{
		{
			iface: "io.ReadWriter",
			want: []Func{
				{
					Name:   "Read",
					Params: []Param{{Name: "p", Type: "[]byte"}},
					Res: []Param{
						{Name: "n", Type: "int"},
						{Name: "err", Type: "error"},
					},
				},
				{
					Name:   "Write",
					Params: []Param{{Name: "p", Type: "[]byte"}},
					Res: []Param{
						{Name: "n", Type: "int"},
						{Name: "err", Type: "error"},
					},
				},
			},
		},
		{
			iface: "http.ResponseWriter",
			want: []Func{
				{
					Name: "Header",
					Res:  []Param{{Type: "http.Header"}},
				},
				{
					Name:   "Write",
					Params: []Param{{Name: "_", Type: "[]byte"}},
					Res:    []Param{{Type: "int"}, {Type: "error"}},
				},
				{
					Name:   "WriteHeader",
					Params: []Param{{Type: "int", Name: "statusCode"}},
				},
			},
		},
		{
			iface: "http.Handler",
			want: []Func{
				{
					Name: "ServeHTTP",
					Params: []Param{
						{Name: "_", Type: "http.ResponseWriter"},
						{Name: "_", Type: "*http.Request"},
					},
				},
			},
		},
		{
			iface: "ast.Node",
			want: []Func{
				{
					Name: "Pos",
					Res:  []Param{{Type: "token.Pos"}},
				},
				{
					Name: "End",
					Res:  []Param{{Type: "token.Pos"}},
				},
			},
		},
		{
			iface: "cipher.AEAD",
			want: []Func{
				{
					Name: "NonceSize",
					Res:  []Param{{Type: "int"}},
				},
				{
					Name: "Overhead",
					Res:  []Param{{Type: "int"}},
				},
				{
					Name: "Seal",
					Params: []Param{
						{Name: "dst", Type: "[]byte"},
						{Name: "nonce", Type: "[]byte"},
						{Name: "plaintext", Type: "[]byte"},
						{Name: "additionalData", Type: "[]byte"},
					},
					Res: []Param{{Type: "[]byte"}},
				},
				{
					Name: "Open",
					Params: []Param{
						{Name: "dst", Type: "[]byte"},
						{Name: "nonce", Type: "[]byte"},
						{Name: "ciphertext", Type: "[]byte"},
						{Name: "additionalData", Type: "[]byte"},
					},
					Res: []Param{{Type: "[]byte"}, {Type: "error"}},
				},
			},
		},
		{
			iface: "error",
			want: []Func{
				{
					Name: "Error",
					Res:  []Param{{Type: "string"}},
				},
			},
		},
		{
			iface: "error",
			want: []Func{
				{
					Name: "Error",
					Res:  []Param{{Type: "string"}},
				},
			},
		},
		{
			iface: "http.Flusher",
			want: []Func{
				{
					Name:     "Flush",
					Comments: "// Flush sends any buffered data to the client.\n",
				},
			},
		},
		{
			iface: "net.Listener",
			want: []Func{
				{
					Name: "Accept",
					Res:  []Param{{Type: "net.Conn"}, {Type: "error"}},
				},
				{
					Name: "Close",
					Res:  []Param{{Type: "error"}},
				},
				{
					Name: "Addr",
					Res:  []Param{{Type: "net.Addr"}},
				},
			},
		},
		{iface: "net.Tennis", wantErr: true},
	}

	for _, tt := range cases {
		fns, err := funcs(tt.iface, "")
		gotErr := err != nil
		if tt.wantErr != gotErr {
			t.Errorf("funcs(%q).err=%v want %s", tt.iface, err, errBool(tt.wantErr))
			continue
		}

		if len(fns) != len(tt.want) {
			t.Errorf("funcs(%q).fns=\n%v\nwant\n%v\n", tt.iface, fns, tt.want)
		}
		for i, fn := range fns {
			if fn.Name != tt.want[i].Name ||
				!reflect.DeepEqual(fn.Params, tt.want[i].Params) ||
				!reflect.DeepEqual(fn.Res, tt.want[i].Res) {

				t.Errorf("funcs(%q).fns=\n%v\nwant\n%v\n", tt.iface, fns, tt.want)
			}
		}
		continue
	}
}

func TestValidReceiver(t *testing.T) {
	cases := []struct {
		recv string
		want bool
	}{
		{recv: "f", want: true},
		{recv: "F", want: true},
		{recv: "f F", want: true},
		{recv: "f *F", want: true},
		{recv: "", want: false},
		{recv: "a+b", want: false},
	}

	for _, tt := range cases {
		got := validReceiver(tt.recv)
		if got != tt.want {
			t.Errorf("validReceiver(%q)=%t want %t", tt.recv, got, tt.want)
		}
	}
}

func TestValidMethodComments(t *testing.T) {
	cases := []struct {
		iface string
		want  []Func
	}{
		{
			iface: "github.com/josharian/impl/testdata.Interface1",
			want: []Func{
				Func{
					Name: "Method1",
					Params: []Param{
						Param{
							Name: "arg1",
							Type: "string",
						}, Param{
							Name: "arg2",
							Type: "string",
						}},
					Res: []Param{
						Param{
							Name: "result",
							Type: "string",
						},
						Param{
							Name: "err",
							Type: "error",
						},
					}, Comments: "// Method1 is the first method of Interface1.\n",
				},
				Func{
					Name: "Method2",
					Params: []Param{
						Param{
							Name: "arg1",
							Type: "int",
						},
						Param{
							Name: "arg2",
							Type: "int",
						},
					},
					Res: []Param{
						Param{
							Name: "result",
							Type: "int",
						},
						Param{
							Name: "err",
							Type: "error",
						},
					},
					Comments: "// Method2 is the second method of Interface1.\n",
				},
				Func{
					Name: "Method3",
					Params: []Param{
						Param{
							Name: "arg1",
							Type: "bool",
						},
						Param{
							Name: "arg2",
							Type: "bool",
						},
					},
					Res: []Param{
						Param{
							Name: "result",
							Type: "bool",
						},
						Param{
							Name: "err",
							Type: "error",
						},
					},
					Comments: "// Method3 is the third method of Interface1.\n",
				},
			},
		},
		{
			iface: "github.com/josharian/impl/testdata.Interface2",
			want: []Func{
				Func{
					Name: "Method1",
					Params: []Param{
						Param{
							Name: "arg1",
							Type: "int64",
						},
						Param{
							Name: "arg2",
							Type: "int64",
						},
					},
					Res: []Param{
						Param{
							Name: "result",
							Type: "int64",
						},
						Param{
							Name: "err",
							Type: "error",
						},
					},
					Comments: "/*\n\t\tMethod1 is the first method of Interface2.\n\t*/\n",
				},
				Func{
					Name: "Method2",
					Params: []Param{
						Param{
							Name: "arg1",
							Type: "float64",
						},
						Param{
							Name: "arg2",
							Type: "float64",
						},
					},
					Res: []Param{
						Param{
							Name: "result",
							Type: "float64",
						},
						Param{
							Name: "err",
							Type: "error",
						},
					},
					Comments: "/*\n\t\tMethod2 is the second method of Interface2.\n\t*/\n",
				},
				Func{
					Name: "Method3",
					Params: []Param{
						Param{
							Name: "arg1",
							Type: "interface{}",
						},
						Param{
							Name: "arg2",
							Type: "interface{}",
						},
					},
					Res: []Param{
						Param{
							Name: "result",
							Type: "interface{}",
						},
						Param{
							Name: "err",
							Type: "error",
						},
					},
					Comments: "/*\n\t\tMethod3 is the third method of Interface2.\n\t*/\n",
				},
			},
		},
		{
			iface: "github.com/josharian/impl/testdata.Interface3",
			want: []Func{
				Func{
					Name: "Method1",
					Params: []Param{
						Param{
							Name: "_",
							Type: "string",
						}, Param{
							Name: "_",
							Type: "string",
						}},
					Res: []Param{
						Param{
							Name: "",
							Type: "string",
						},
						Param{
							Name: "",
							Type: "error",
						},
					}, Comments: "// Method1 is the first method of Interface3.\n",
				},
				Func{
					Name: "Method2",
					Params: []Param{
						Param{
							Name: "_",
							Type: "int",
						},
						Param{
							Name: "arg2",
							Type: "int",
						},
					},
					Res: []Param{
						Param{
							Name: "_",
							Type: "int",
						},
						Param{
							Name: "err",
							Type: "error",
						},
					},
					Comments: "// Method2 is the second method of Interface3.\n",
				},
				Func{
					Name: "Method3",
					Params: []Param{
						Param{
							Name: "arg1",
							Type: "bool",
						},
						Param{
							Name: "arg2",
							Type: "bool",
						},
					},
					Res: []Param{
						Param{
							Name: "result1",
							Type: "bool",
						},
						Param{
							Name: "result2",
							Type: "bool",
						},
					},
					Comments: "// Method3 is the third method of Interface3.\n",
				},
			},
		},
	}

	for _, tt := range cases {
		fns, err := funcs(tt.iface, ".")
		if err != nil {
			t.Errorf("funcs(%q).err=%v", tt.iface, err)
		}
		if !reflect.DeepEqual(fns, tt.want) {
			t.Errorf("funcs(%q).fns=\n%v\nwant\n%v\n", tt.iface, fns, tt.want)
		}
	}
}

func TestStubGeneration(t *testing.T) {
	cases := []struct {
		iface string
		want  string
	}{
		{
			iface: "github.com/josharian/impl/testdata.Interface1",
			want:  testdata.Interface1Output,
		},
		{
			iface: "github.com/josharian/impl/testdata.Interface2",
			want:  testdata.Interface2Output,
		},
		{
			iface: "github.com/josharian/impl/testdata.Interface3",
			want:  testdata.Interface3Output,
		},
	}
	for _, tt := range cases {
		fns, err := funcs(tt.iface, ".")
		if err != nil {
			t.Errorf("funcs(%q).err=%v", tt.iface, err)
		}
		src := genStubs("r *Receiver", fns)
		if string(src) != tt.want {
			t.Errorf("genStubs(\"r *Receiver\", %+#v).src=\n%s\nwant\n%s\n", fns, string(src), tt.want)
		}
	}
}
