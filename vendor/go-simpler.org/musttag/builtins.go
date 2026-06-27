package musttag

// builtins is a set of functions supported out of the box.
var builtins = []Func{
	// https://pkg.go.dev/encoding/json
	{"encoding/json.Marshal", "json", 0, []string{"encoding/json.Marshaler", "encoding.TextMarshaler"}},
	{"encoding/json.MarshalIndent", "json", 0, []string{"encoding/json.Marshaler", "encoding.TextMarshaler"}},
	{"encoding/json.Unmarshal", "json", 1, []string{"encoding/json.Unmarshaler", "encoding.TextUnmarshaler"}},
	{"(*encoding/json.Encoder).Encode", "json", 0, []string{"encoding/json.Marshaler", "encoding.TextMarshaler"}},
	{"(*encoding/json.Decoder).Decode", "json", 0, []string{"encoding/json.Unmarshaler", "encoding.TextUnmarshaler"}},

	// https://pkg.go.dev/encoding/xml
	{"encoding/xml.Marshal", "xml", 0, []string{"encoding/xml.Marshaler", "encoding.TextMarshaler"}},
	{"encoding/xml.MarshalIndent", "xml", 0, []string{"encoding/xml.Marshaler", "encoding.TextMarshaler"}},
	{"encoding/xml.Unmarshal", "xml", 1, []string{"encoding/xml.Unmarshaler", "encoding.TextUnmarshaler"}},
	{"(*encoding/xml.Encoder).Encode", "xml", 0, []string{"encoding/xml.Marshaler", "encoding.TextMarshaler"}},
	{"(*encoding/xml.Decoder).Decode", "xml", 0, []string{"encoding/xml.Unmarshaler", "encoding.TextUnmarshaler"}},
	{"(*encoding/xml.Encoder).EncodeElement", "xml", 0, []string{"encoding/xml.Marshaler", "encoding.TextMarshaler"}},
	{"(*encoding/xml.Decoder).DecodeElement", "xml", 0, []string{"encoding/xml.Unmarshaler", "encoding.TextUnmarshaler"}},

	// https://pkg.go.dev/gopkg.in/yaml.v3
	{"gopkg.in/yaml.v3.Marshal", "yaml", 0, []string{"gopkg.in/yaml.v3.Marshaler", "encoding.TextMarshaler"}},
	{"gopkg.in/yaml.v3.Unmarshal", "yaml", 1, []string{"gopkg.in/yaml.v3.Unmarshaler", "encoding.TextUnmarshaler"}},
	{"(*gopkg.in/yaml.v3.Encoder).Encode", "yaml", 0, []string{"gopkg.in/yaml.v3.Marshaler", "encoding.TextMarshaler"}},
	{"(*gopkg.in/yaml.v3.Decoder).Decode", "yaml", 0, []string{"gopkg.in/yaml.v3.Unmarshaler", "encoding.TextUnmarshaler"}},

	// https://pkg.go.dev/github.com/BurntSushi/toml
	{"github.com/BurntSushi/toml.Unmarshal", "toml", 1, []string{"github.com/BurntSushi/toml.Unmarshaler", "encoding.TextUnmarshaler"}},
	{"github.com/BurntSushi/toml.Decode", "toml", 1, []string{"github.com/BurntSushi/toml.Unmarshaler", "encoding.TextUnmarshaler"}},
	{"github.com/BurntSushi/toml.DecodeFS", "toml", 2, []string{"github.com/BurntSushi/toml.Unmarshaler", "encoding.TextUnmarshaler"}},
	{"github.com/BurntSushi/toml.DecodeFile", "toml", 1, []string{"github.com/BurntSushi/toml.Unmarshaler", "encoding.TextUnmarshaler"}},
	{"(*github.com/BurntSushi/toml.Encoder).Encode", "toml", 0, []string{"encoding.TextMarshaler"}},
	{"(*github.com/BurntSushi/toml.Decoder).Decode", "toml", 0, []string{"github.com/BurntSushi/toml.Unmarshaler", "encoding.TextUnmarshaler"}},

	// https://pkg.go.dev/github.com/mitchellh/mapstructure
	{"github.com/mitchellh/mapstructure.Decode", "mapstructure", 1, nil},
	{"github.com/mitchellh/mapstructure.DecodeMetadata", "mapstructure", 1, nil},
	{"github.com/mitchellh/mapstructure.WeakDecode", "mapstructure", 1, nil},
	{"github.com/mitchellh/mapstructure.WeakDecodeMetadata", "mapstructure", 1, nil},

	// https://pkg.go.dev/github.com/jmoiron/sqlx
	{"github.com/jmoiron/sqlx.Get", "db", 1, []string{"database/sql.Scanner"}},
	{"github.com/jmoiron/sqlx.GetContext", "db", 2, []string{"database/sql.Scanner"}},
	{"github.com/jmoiron/sqlx.Select", "db", 1, []string{"database/sql.Scanner"}},
	{"github.com/jmoiron/sqlx.SelectContext", "db", 2, []string{"database/sql.Scanner"}},
	{"github.com/jmoiron/sqlx.StructScan", "db", 1, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.Conn).GetContext", "db", 1, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.Conn).SelectContext", "db", 1, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.DB).Get", "db", 0, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.DB).GetContext", "db", 1, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.DB).Select", "db", 0, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.DB).SelectContext", "db", 1, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.NamedStmt).Get", "db", 0, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.NamedStmt).GetContext", "db", 1, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.NamedStmt).Select", "db", 0, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.NamedStmt).SelectContext", "db", 1, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.Row).StructScan", "db", 0, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.Rows).StructScan", "db", 0, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.Stmt).Get", "db", 0, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.Stmt).GetContext", "db", 1, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.Stmt).Select", "db", 0, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.Stmt).SelectContext", "db", 1, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.Tx).Get", "db", 0, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.Tx).GetContext", "db", 1, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.Tx).Select", "db", 0, []string{"database/sql.Scanner"}},
	{"(*github.com/jmoiron/sqlx.Tx).SelectContext", "db", 1, []string{"database/sql.Scanner"}},
}
