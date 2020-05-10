package gotests

import (
	"errors"
	"go/types"
	"io/ioutil"
	"path"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/tools/imports"
)

func TestGenerateTests(t *testing.T) {
	type args struct {
		srcPath     string
		only        *regexp.Regexp
		excl        *regexp.Regexp
		exported    bool
		printInputs bool
		subtests    bool
		importer    types.Importer
		templateDir string
	}
	tests := []struct {
		name              string
		args              args
		want              string
		wantNoTests       bool
		wantMultipleTests bool
		wantErr           bool
	}{
		{
			name: "Blank Go file",
			args: args{
				srcPath: `testdata/blankfile/blank.go`,
			},
			wantNoTests: true,
			wantErr:     true,
		}, {
			name: "Blank Go file in directory",
			args: args{
				srcPath: `testdata/blankfile/notblank.go`,
			},
			wantNoTests: true,
			wantErr:     true,
		}, {
			name: "Test file with garbage data",
			args: args{
				srcPath: `testdata/invalidtest/invalid.go`,
			},
			wantNoTests: true,
			wantErr:     true,
		}, {
			name: "Hidden file",
			args: args{
				srcPath: `testdata/.hidden.go`,
			},
			wantNoTests: true,
			wantErr:     true,
		}, {
			name: "Nonexistant file",
			args: args{
				srcPath: `testdata/nonexistant.go`,
			},
			wantNoTests: true,
			wantErr:     true,
		}, {
			name: "Target test file",
			args: args{
				srcPath:  `testdata/test103_test.go`,
				only:     regexp.MustCompile("wrapToString"),
				subtests: true,
			},
			wantNoTests: false,
			wantErr:     false,
			want:        mustReadAndFormatGoFile(t, `testdata/goldens/target_test_file.go`),
		}, {
			name: "Target test file without only flag",
			args: args{
				srcPath:  `testdata/test103_test.go`,
				subtests: true,
			},
			wantNoTests: false,
			wantErr:     false,
			want:        mustReadAndFormatGoFile(t, `testdata/goldens/target_test_file.go`),
		}, {
			name: "No funcs",
			args: args{
				srcPath: `testdata/test000.go`,
			},
			wantNoTests: true,
		}, {
			name: "Function with neither receiver, parameters, nor results",
			args: args{
				srcPath: `testdata/test001.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_neither_receiver_parameters_nor_results.go"),
		}, {
			name: "Function with anonymous arguments",
			args: args{
				srcPath: `testdata/test002.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_anonymous_arguments.go"),
		}, {
			name: "Function with named argument",
			args: args{
				srcPath: `testdata/test003.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_named_argument.go"),
		}, {
			name: "Function with return value",
			args: args{
				srcPath: `testdata/test004.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_return_value.go"),
		}, {
			name: "Function returning an error",
			args: args{
				srcPath: `testdata/test005.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_returning_an_error.go"),
		}, {
			name: "Function with multiple arguments",
			args: args{
				srcPath: `testdata/test006.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_multiple_arguments.go"),
		}, {
			name: "Print inputs with multiple arguments",
			args: args{
				srcPath:     `testdata/test006.go`,
				printInputs: true,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/print_inputs_with_multiple_arguments.go"),
		}, {
			name: "Method on a struct pointer",
			args: args{
				srcPath: `testdata/test007.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/method_on_a_struct_pointer.go"),
		}, {
			name: "Print inputs with single argument",
			args: args{
				srcPath:     `testdata/test007.go`,
				printInputs: true,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/print_inputs_with_single_argument.go"),
		}, {
			name: "Function with struct pointer argument and return type",
			args: args{
				srcPath: `testdata/test008.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_struct_pointer_argument_and_return_type.go"),
		}, {
			name: "Struct value method with struct value return type",
			args: args{
				srcPath: `testdata/test009.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/struct_value_method_with_struct_value_return_type.go"),
		}, {
			name: "Function with map argument and return type",
			args: args{
				srcPath: `testdata/test010.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_map_argument_and_return_type.go"),
		}, {
			name: "Function with slice argument and return type",
			args: args{
				srcPath: `testdata/test011.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_slice_argument_and_return_type.go"),
		}, {
			name: "Function returning only an error",
			args: args{
				srcPath: `testdata/test012.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_returning_only_an_error.go"),
		}, {
			name: "Function with a function parameter",
			args: args{
				srcPath: `testdata/test013.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_a_function_parameter.go"),
		}, {
			name: "Function with a function parameter with its own parameters and result",
			args: args{
				srcPath: `testdata/test014.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_a_function_parameter_with_its_own_parameters_and_result.go"),
		}, {
			name: "Function with a function parameter that returns two results",
			args: args{
				srcPath: `testdata/test015.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_a_function_parameter_that_returns_two_results.go"),
		}, {
			name: "Function with defined interface type parameter and result",
			args: args{
				srcPath: `testdata/test016.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_defined_interface_type_parameter_and_result.go"),
		}, {
			name: "Function with imported interface receiver, parameter, and result",
			args: args{
				srcPath: `testdata/test017.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_imported_interface_receiver_parameter_and_result.go"),
		}, {
			name: "Function with imported struct receiver, parameter, and result",
			args: args{
				srcPath: `testdata/test018.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_imported_struct_receiver_parameter_and_result.go"),
		}, {
			name: "Function with multiple parameters of the same type",
			args: args{
				srcPath: `testdata/test019.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_multiple_parameters_of_the_same_type.go"),
		}, {
			name: "Function with a variadic parameter",
			args: args{
				srcPath: `testdata/test020.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_a_variadic_parameter.go"),
		}, {
			name: "Function with interface{} parameter and result",
			args: args{
				srcPath: `testdata/test021.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_interface_parameter_and_result.go"),
		}, {
			name: "Function with named imports",
			args: args{
				srcPath: `testdata/test022.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_named_imports.go"),
		}, {
			name: "Function with channel parameter and result",
			args: args{
				srcPath: `testdata/test023.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_channel_parameter_and_result.go"),
		}, {
			name: "File with multiple imports",
			args: args{
				srcPath: `testdata/test024.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/file_with_multiple_imports.go"),
		}, {
			name: "Function returning two results and an error",
			args: args{
				srcPath: `testdata/test025.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_returning_two_results_and_an_error.go"),
		}, {
			name: "Multiple named results",
			args: args{
				srcPath: `testdata/test026.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_named_results.go"),
		}, {
			name: "Two different structs with same method name",
			args: args{
				srcPath: `testdata/test027.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/two_different_structs_with_same_method_name.go"),
		}, {
			name: "Underlying types",
			args: args{
				srcPath: `testdata/test028.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/underlying_types.go"),
		}, {
			name: "Struct receiver with multiple fields",
			args: args{
				srcPath: `testdata/test029.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/struct_receiver_with_multiple_fields.go"),
		}, {
			name: "Struct receiver with anonymous fields",
			args: args{
				srcPath: `testdata/test030.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/struct_receiver_with_anonymous_fields.go"),
		}, {
			name: "io.Writer parameters",
			args: args{
				srcPath: `testdata/test031.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/io_writer_parameters.go"),
		}, {
			name: "Two structs with same method name",
			args: args{
				srcPath: `testdata/test032.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/two_structs_with_same_method_name.go"),
		}, {
			name: "Functions and methods with 'name' receivers, parameters, and results",
			args: args{
				srcPath: `testdata/test033.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/functions_and_methods_with_name_receivers_parameters_and_results.go"),
		}, {
			name: "Receiver struct with reserved field names",
			args: args{
				srcPath: `testdata/test034.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/receiver_struct_with_reserved_field_names.go"),
		}, {
			name: "Receiver struct with fields with complex package names",
			args: args{
				srcPath: `testdata/test035.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/receiver_struct_with_fields_with_complex_package_names.go"),
		}, {
			name: "Functions and receivers with same names except exporting",
			args: args{
				srcPath: `testdata/test036.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/functions_and_receivers_with_same_names_except_exporting.go"),
		}, {
			name: "Receiver is indirect imported struct",
			args: args{
				srcPath: `testdata/test037.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/receiver_is_indirect_imported_struct.go"),
		}, {
			name: "Multiple functions",
			args: args{
				srcPath: `testdata/test_filter.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_functions.go"),
		}, {
			name: "Multiple functions with only",
			args: args{
				srcPath: `testdata/test_filter.go`,
				only:    regexp.MustCompile("FooFilter|bazFilter"),
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_functions_with_only.go"),
		}, {
			name: "Multiple functions with only regexp without matches",
			args: args{
				srcPath: `testdata/test_filter.go`,
				only:    regexp.MustCompile("asdf"),
			},
			wantNoTests: true,
		}, {
			name: "Multiple functions with case-insensitive only",
			args: args{
				srcPath: `testdata/test_filter.go`,
				only:    regexp.MustCompile("(?i)fooFilter|BazFilter"),
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_functions_with_case-insensitive_only.go"),
		}, {
			name: "Multiple functions with only filtering on receiver",
			args: args{
				srcPath: `testdata/test_filter.go`,
				only:    regexp.MustCompile("^BarBarFilter$"),
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_functions_with_only_filtering_on_receiver.go"),
		}, {
			name: "Multiple functions with only filtering on method",
			args: args{
				srcPath: `testdata/test_filter.go`,
				only:    regexp.MustCompile("^(BarFilter)$"),
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_functions_with_only_filtering_on_method.go"),
		}, {
			name: "Multiple functions filtering exported",
			args: args{
				srcPath:  `testdata/test_filter.go`,
				exported: true,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_functions_filtering_exported.go"),
		}, {
			name: "Multiple functions filtering exported with only",
			args: args{
				srcPath:  `testdata/test_filter.go`,
				only:     regexp.MustCompile(`FooFilter`),
				exported: true,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_functions_filtering_exported_with_only.go"),
		}, {
			name: "Multiple functions filtering all out",
			args: args{
				srcPath: `testdata/test_filter.go`,
				only:    regexp.MustCompile("fooFilter"),
			},
			wantNoTests: true,
		}, {
			name: "Multiple functions with excl",
			args: args{
				srcPath: `testdata/test_filter.go`,
				excl:    regexp.MustCompile("FooFilter|bazFilter"),
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_functions_with_excl.go"),
		}, {
			name: "Multiple functions with case-insensitive excl",
			args: args{
				srcPath: `testdata/test_filter.go`,
				excl:    regexp.MustCompile("(?i)foOFilter|BaZFilter"),
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_functions_with_case-insensitive_excl.go"),
		}, {
			name: "Multiple functions filtering exported with excl",
			args: args{
				srcPath:  `testdata/test_filter.go`,
				excl:     regexp.MustCompile(`FooFilter`),
				exported: true,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_functions_filtering_exported_with_excl.go"),
		}, {
			name: "Multiple functions excluding all",
			args: args{
				srcPath: `testdata/test_filter.go`,
				excl:    regexp.MustCompile("bazFilter|FooFilter|BarFilter"),
			},
			wantNoTests: true,
		}, {
			name: "Multiple functions excluding on receiver",
			args: args{
				srcPath: `testdata/test_filter.go`,
				excl:    regexp.MustCompile("^BarBarFilter$"),
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_functions_excluding_on_receiver.go"),
		}, {
			name: "Multiple functions excluding on method",
			args: args{
				srcPath: `testdata/test_filter.go`,
				excl:    regexp.MustCompile("^BarFilter$"),
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_functions_excluding_on_method.go"),
		}, {
			name: "Multiple functions with both only and excl",
			args: args{
				srcPath: `testdata/test_filter.go`,
				only:    regexp.MustCompile("BarFilter"),
				excl:    regexp.MustCompile("FooFilter"),
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_functions_with_both_only_and_excl.go"),
		}, {
			name: "Multiple functions with only and excl competing",
			args: args{
				srcPath: `testdata/test_filter.go`,
				only:    regexp.MustCompile("FooFilter|BarFilter"),
				excl:    regexp.MustCompile("FooFilter|bazFilter"),
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/multiple_functions_with_only_and_excl_competing.go"),
		}, {
			name: "Custom importer fails",
			args: args{
				srcPath:  `testdata/test_filter.go`,
				importer: &fakeImporter{err: errors.New("error")},
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/custom_importer_fails.go"),
		}, {
			name: "Existing test file",
			args: args{
				srcPath: `testdata/test100.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/existing_test_file.go"),
		}, {
			name: "Existing test file with just package declaration",
			args: args{
				srcPath: `testdata/test101.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/existing_test_file_with_just_package_declaration.go"),
		}, {
			name: "Existing test file with no functions",
			args: args{
				srcPath: `testdata/test102.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/existing_test_file_with_no_functions.go"),
		}, {
			name: "Existing test file with multiple imports",
			args: args{
				srcPath: `testdata/test200.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/existing_test_file_with_multiple_imports.go"),
		}, {
			name: "Entire testdata directory",
			args: args{
				srcPath: `testdata/`,
			},
			wantMultipleTests: true,
		}, {
			name: "Different packages in same directory - part 1",
			args: args{
				srcPath: `testdata/mixedpkg/bar.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/different_packages_in_same_directory_-_part_1.go"),
		}, {
			name: "Different packages in same directory - part 2",
			args: args{
				srcPath: `testdata/mixedpkg/foo.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/different_packages_in_same_directory_-_part_2.go"),
		}, {
			name: "Empty test file",
			args: args{
				srcPath: `testdata/blanktest/blank.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/empty_test_file.go"),
		}, {
			name: "Test file with syntax errors",
			args: args{
				srcPath: `testdata/syntaxtest/syntax.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/test_file_with_syntax_errors.go"),
		}, {
			name: "Undefined types",
			args: args{
				srcPath: `testdata/undefinedtypes/undefined.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/undefined_types.go"),
		}, {
			name: "Subtest Edition - Functions and receivers with same names except exporting",
			args: args{
				srcPath:  `testdata/test036.go`,
				subtests: true,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/subtest_edition_-_functions_and_receivers_with_same_names_except_exporting.go"),
		},
		{
			name: "Init function",
			args: args{
				srcPath: `testdata/init_func.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/no_init_funcs.go"),
		},
		{
			name: "Existing test file with package level comments",
			args: args{
				srcPath: `testdata/test_existing_test_file_with_comments.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/existing_test_file_with_package_level_comments.go"),
		},
		{
			name: "Existing test file with package level comments with newline",
			args: args{
				srcPath: `testdata/test_existing_test_file_with_comments.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/existing_test_file_with_package_level_comments.go"),
		},
		{
			name: "Existing test file with package level comments without newline",
			args: args{
				srcPath: `testdata/test_existing_test_file_wih_comments_without_newline.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/existing_test_file_with_package_level_comments_without_newline.go"),
		},
		{
			name: "Existing test file with mixed types package level comments",
			args: args{
				srcPath: `testdata/test_existing_test_file_wih_mixed_comments.go`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/existing_test_file_with_package_level_mixed_types_comments.go"),
		},
		{
			name: "Naked function with subtests",
			args: args{
				srcPath:  "testdata/naked_function_with_subtests.go",
				subtests: true,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/naked_function_with_subtests.go"),
		},
		{
			name: "Naked function without subtests",
			args: args{
				srcPath:  "testdata/naked_function_without_subtests.go",
				subtests: false,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/naked_function_without_subtests.go"),
		},
		{
			name: "Test non existing template path",
			args: args{
				srcPath:     `testdata/calculator.go`,
				templateDir: `/tmp/not/exising/path`,
			},
			wantErr:     true,
			wantNoTests: true,
		},
		{
			name: "Test non bad template formatting",
			args: args{
				srcPath:     `testdata/calculator.go`,
				templateDir: `testdata/bad_customtemplates`,
			},
			wantErr:     true,
			wantNoTests: true,
		},
		{
			name: "Test custom template path",
			args: args{
				srcPath:     `testdata/test004.go`,
				templateDir: `testdata/customtemplates`,
			},
			want: mustReadAndFormatGoFile(t, "testdata/goldens/function_with_return_value_custom_template.go"),
		},
		{
			name: "Test interface embedding",
			args: args{
				srcPath: `testdata/undefinedtypes/interface_embedding.go`,
			},
			want:        mustReadAndFormatGoFile(t, "testdata/goldens/interface_embedding.go"),
			wantNoTests: !versionGreaterOrEqualThan("go1.11"),
			wantErr:     !versionGreaterOrEqualThan("go1.11"),
		},
	}
	tmp, err := ioutil.TempDir("", "gotests_test")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	for _, tt := range tests {
		gts, err := GenerateTests(tt.args.srcPath, &Options{
			Only:        tt.args.only,
			Exclude:     tt.args.excl,
			Exported:    tt.args.exported,
			PrintInputs: tt.args.printInputs,
			Subtests:    tt.args.subtests,
			Importer:    func() types.Importer { return tt.args.importer },
			TemplateDir: tt.args.templateDir,
		})
		if (err != nil) != tt.wantErr {
			t.Errorf("%q. GenerateTests(%v) error = %v, wantErr %v", tt.name, tt.args.srcPath, err, tt.wantErr)
			continue
		}
		if (len(gts) == 0) != tt.wantNoTests {
			t.Errorf("%q. GenerateTests(%v) returned no tests", tt.name, tt.args.srcPath)
			continue
		}
		if (len(gts) > 1) != tt.wantMultipleTests {
			t.Errorf("%q. GenerateTests(%v) returned too many tests", tt.name, tt.args.srcPath)
			continue
		}
		if tt.wantNoTests || tt.wantMultipleTests {
			continue
		}
		if got := string(gts[0].Output); got != tt.want {
			t.Errorf("%q. GenerateTests(%v) = \n%v, want \n%v", tt.name, tt.args.srcPath, got, tt.want)
			outputResult(t, tmp, tt.name, gts[0].Output)
		}
	}
}

func versionGreaterOrEqualThan(version string) bool {
	prefixes := []string{"go1.9", "go1.10", "go1.11", "go1.12", "go1.13"}
	v := runtime.Version()
	for _, prefix := range prefixes {
		if strings.Contains(version, prefix) {
			return true
		}
		if strings.Contains(v, prefix) {
			return false
		}
	}
	return true
}

func mustReadAndFormatGoFile(t *testing.T, filename string) string {
	fmted, err := imports.Process(filename, nil, nil)
	if err != nil {
		t.Fatalf("reading and formatting file: %v", err)
	}
	return string(fmted)
}

func outputResult(t *testing.T, tmpDir, testName string, got []byte) {
	tmpResult := path.Join(tmpDir, toSnakeCase(testName)+".go")
	if err := ioutil.WriteFile(tmpResult, got, 0644); err != nil {
		t.Errorf("ioutil.WriteFile: %v", err)
	}
	t.Logf(tmpResult)
}

func toSnakeCase(s string) string {
	var res []rune
	for _, r := range []rune(s) {
		r = unicode.ToLower(r)
		switch r {
		case ' ', '.':
			r = '_'
		case ',', '\'', '{', '}':
			continue
		}
		res = append(res, r)
	}
	return string(res)
}

// 249032394 ns/op
func BenchmarkGenerateTests(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateTests("testdata/", &Options{})
	}
}

// A fake importer.
type fakeImporter struct {
	err error
}

func (f *fakeImporter) Import(path string) (*types.Package, error) {
	return nil, f.err
}
