package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"os"

	"github.com/coreos/etcdctl/third_party/github.com/codegangsta/cli"
	"github.com/coreos/etcdctl/third_party/github.com/coreos/go-etcd/etcd"
	"io/ioutil"
	"io"
)

// Import takes a JSON file and attempts to import it into Etcd. For given JSON like
// {
//   "base": {
//     "inner": "inner_value"
//   }
//
// It will first create a directory "base", and then create a key within base called "inner" with the value
// "inner_value"


// NewImportCommand returns the CLI command for "import"
func NewImportCommand() cli.Command {
	return cli.Command{
		Name:  "import",
		Usage: "import a JSON file into the etcd tree",
		Description: "The command accepts a single argument, which is the file from which to import.",
		Flags: []cli.Flag{
			cli.BoolFlag{"dry-run, n", "No action. Simulate what actions would be performed"},
			cli.BoolFlag{"overwrite, o", "Overwrite keys if they already exist"},
			cli.BoolFlag{"quiet, q", "Do not print output"},
		},
		Action: func(c *cli.Context) {
			handleAll(c, handleImport)
		},
	}
}

// handleImport takes a list of JSON files and imports them into the Etcd tree
func handleImport(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	if !c.Args().Present() {
		handleError(MalformedEtcdctlArguments, ErrNumArgs)
	}

	for _, path := range c.Args() {
		if stats, err := os.Stat(path); err != nil || stats.IsDir() {
			handleError(MalformedEtcdctlArguments, ErrNotFile)
		}
	}

	i := &JsonImporter{
		client: client,
		context: c,
		Overwrite: c.Bool("overwrite"),
		NoOp: c.Bool("dry-run"),
		Quiet: c.Bool("quiet"),
	}

	for _, path := range c.Args() {
		if err := i.Import(path); err != nil {
			handleError(ErrorFromEtcd, err)
		}
	}
	return client.Get("/", false, true)
}

type JsonImporter struct {
	client *etcd.Client // etcd client
	context *cli.Context // cli Context.
	NoOp bool // No-operation. Go through procedure but do not make any write calls
	Overwrite bool // Overwrite existing keys
	Quiet bool // Do not print output
}

// Import takes a JSON file and imports it into the Etcd structure.
func (self *JsonImporter) Import(json_file string) (err error) {
	var json_data io.Reader
	var byte_data []byte

	if byte_data, err = ioutil.ReadFile(json_file); err != nil {
		handleError(MalformedEtcdctlArguments, err)
	}
	json_data = bytes.NewReader(byte_data)

	d := json.NewDecoder(json_data)
	for {
		var v map[string]interface{}
		if err = d.Decode(&v); err == io.EOF {
			err = nil
			break
		} else if err != nil {
			break
		}
		if err = self.parseDeepElement("/", v); err != nil {
			break
		}
	}
	return err
}

// parseDeepElement parses a JSON element, determines whether it is a string, number or an interface. If it is a number
// or string the key is added via `putValue`. If the element is a directory the directory is added via `putDirectory`
// and then parseDeepElement is recursively called
func (self *JsonImporter) parseDeepElement(base_path string, element map[string]interface{}) (output_error error) {
	var full_key string
	var value string

	for key, raw_value := range element {
		full_key = path.Join(base_path, key)
		switch raw_value.(type) {
		case string:
			value = raw_value.(string)
		case float64:
			value = fmt.Sprintf("%.f", raw_value.(float64))
		case map[string]interface{}:
			if err := self.putDirectory(full_key); err != nil {
				output_error = err
				break
			}
			if err := self.parseDeepElement(full_key, raw_value.(map[string]interface{})); err != nil {
				output_error = err
				break
			}
			continue
		default:
			fmt.Println("I don't know how to handle", key)
			continue
		}

		if err := self.putValue(full_key, value); err != nil {
			output_error = err
			break
		}
	}
	return output_error
}

// putValue takes a key and value and adds it to the Etcd tree, if NoOp is set to false. If Overwrite is set to
// true the value will be set, if false the value will be put. If set to quiet no output will be printed
func (self *JsonImporter) putValue(key, value string) (output_error error) {
	str := fmt.Sprintf("PUT %s to %s", key, value)
	if !self.NoOp {
		if self.Overwrite {
			str = fmt.Sprintf("SET %s to %s", key, value)
			_, output_error = self.client.Set(key, value, 0)
		} else {
			_, output_error = self.client.Create(key, value, 0)
		}
	}

	if output_error == nil {
		self.Print(str)
	}
	return output_error
}

// putDirectory takes a key and value and adds it to the Etcd tree, if NoOp is set to false. If Overwrite is set to
// true the value will be set, if false the value will be put. If set to quiet no output will be printed
func (self *JsonImporter) putDirectory(key string) (output_error error) {
	str := fmt.Sprintf("CREATE DIRECTORY %s", key)

	if !self.NoOp {
		if self.Overwrite {
			if _, output_error := self.client.SetDir(key, 0); output_error!= nil {
				if resp, g_err := self.client.Get(key, false, false); g_err != nil {
					output_error = g_err
				} else if resp.Node.Dir {
					str = fmt.Sprintf("DIRECTORY %s exists", key)
					output_error = nil
				}
			}
		} else {
			_, output_error = self.client.CreateDir(key, 0)
		}
	}
	if output_error == nil {
		self.Print(str)
	}
	return output_error
}

// Print outputs a string, depending on whether the Quiet switch has been set
func (self *JsonImporter) Print(str string) {
	if !self.Quiet {
		fmt.Println(str)
	}
}
