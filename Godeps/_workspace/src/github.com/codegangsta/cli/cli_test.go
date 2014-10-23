package cli_test

import (
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"os"
)

func Example() {
	app := cli.NewApp()
	app.Name = "todo"
	app.Usage = "task list on the command line"
	app.Commands = []cli.Command{
		{
			Name:      "add",
			ShortName: "a",
			Usage:     "add a task to the list",
			Action: func(c *cli.Context) {
				println("added task: ", c.Args().First())
			},
		},
		{
			Name:      "complete",
			ShortName: "c",
			Usage:     "complete a task on the list",
			Action: func(c *cli.Context) {
				println("completed task: ", c.Args().First())
			},
		},
	}

	app.Run(os.Args)
}

func ExampleSubcommand() {
	app := cli.NewApp()
	app.Name = "say"
	app.Commands = []cli.Command{
		{
			Name:        "hello",
			ShortName:   "hi",
			Usage:       "use it to see a description",
			Description: "This is how we describe hello the function",
			Subcommands: []cli.Command{
				{
					Name:        "english",
					ShortName:   "en",
					Usage:       "sends a greeting in english",
					Description: "greets someone in english",
					Flags: []cli.Flag{
						cli.StringFlag{"name", "Bob", "Name of the person to greet"},
					},
					Action: func(c *cli.Context) {
						println("Hello, ", c.String("name"))
					},
				}, {
					Name:      "spanish",
					ShortName: "sp",
					Usage:     "sends a greeting in spanish",
					Flags: []cli.Flag{
						cli.StringFlag{"surname", "Jones", "Surname of the person to greet"},
					},
					Action: func(c *cli.Context) {
						println("Hola, ", c.String("surname"))
					},
				}, {
					Name:      "french",
					ShortName: "fr",
					Usage:     "sends a greeting in french",
					Flags: []cli.Flag{
						cli.StringFlag{"nickname", "Stevie", "Nickname of the person to greet"},
					},
					Action: func(c *cli.Context) {
						println("Bonjour, ", c.String("nickname"))
					},
				},
			},
		}, {
			Name:  "bye",
			Usage: "says goodbye",
			Action: func(c *cli.Context) {
				println("bye")
			},
		},
	}

	app.Run(os.Args)
}
