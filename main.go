package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/tmtk75/cli"
	"github.com/vaughan0/go-ini"
)

func main() {
	app := cli.NewApp()
	app.Name = "envaws"
	app.Version = "0.1.0dev"
	app.Usage = `AWS access key manager
	    Help you to export environment variables and unset them easily.

   e.g)
     To export AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY for default profile

        $ eval $(envaws env default)

     To unset them

        $ eval $(envaws unset)`
	app.Commands = []cli.Command{
		cli.Command{
			Name:  "ls",
			Usage: "List available profiles in ~/.aws/credentials",
			Flags: []cli.Flag{},
			Action: func(ctx *cli.Context) {
				listProfiles()
			},
		},
		cli.Command{
			Name:  "env",
			Usage: "Print keys as environtment variables for profile",
			Args:  "<profile>",
			Flags: []cli.Flag{},
			Action: func(ctx *cli.Context) {
				profile, _ := ctx.ArgFor("profile")
				sec := loadSection(profile)
				formatEnv(sec, ctx.String("format"))
			},
		},
		cli.Command{
			Name:  "unset",
			Usage: "Print commands to unset environtment variables for AWS_*",
			Action: func(ctx *cli.Context) {
				unset()
			},
		},
		cli.Command{
			Name:  "tf",
			Args:  "<profile>",
			Usage: "Print keys as terraform variable definition for profile",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "format,f", Value: "var", Usage: "option,env,var,export"},
			},
			Action: func(ctx *cli.Context) {
				profile, _ := ctx.ArgFor("profile")
				sec := loadSection(profile)
				formatTf(sec, ctx.String("format"))
			},
		},
	}
	app.Run(os.Args)
}

const ACCESS_KEY_ID = "aws_access_key_id"
const SECRET_ACCESS_KEY = "aws_secret_access_key"

func loadInifile() ini.File {
	path, err := homedir.Expand("~/.aws/credentials")
	if err != nil {
		log.Fatalf("doesn't exist: %v\n", err)
	}

	if file, err := os.Open(path); err != nil {
		log.Fatalf("failed to open: %v\n", err)
	} else {
		inifile, err := ini.Load(file)
		if err != nil {
			log.Fatalf("%v\n", err)
		}

		return inifile
	}
	// Never reach
	return nil
}

func loadSection(profile string) ini.Section {
	inifile := loadInifile()
	for k, _ := range inifile {
		if k == profile {
			sec := inifile[k]
			if sec[ACCESS_KEY_ID] == "" {
				log.Fatalf("aws_access_key_id is empty")
			}
			if sec[SECRET_ACCESS_KEY] == "" {
				log.Fatalf("aws_secret_access_key is empty")
			}
			return sec
		}
	}
	log.Fatalf("missing %v\n", profile)
	return nil
}

const tf_env = `TF_VAR_aws_access_key="{{.aws_access_key_id}}" TF_VAR_aws_secret_key="{{.aws_secret_access_key}}"`
const tf_var = `
aws_access_key = {{.aws_access_key_id}}
aws_secret_key = {{.aws_secret_access_key}}`
const tf_option = `-var aws_access_key="{{.aws_access_key_id}}" -var aws_secret_key="{{.aws_secret_access_key}}"`
const tf_export = `
export TF_VAR_aws_access_key="{{.aws_access_key_id}}"
export TF_VAR_aws_secret_key="{{.aws_secret_access_key}}"`

func formatTf(sec ini.Section, format string) {
	templ := (map[string]string{
		"env":    tf_env,
		"var":    tf_var,
		"option": tf_option,
		"export": tf_export,
	})[format]
	if templ == "" {
		log.Fatalf("not supported format: %v", format)
	}

	formatTempl(sec, templ)
}

func formatEnv(sec ini.Section, format string) {
	templ := `
export AWS_ACCESS_KEY_ID="{{.aws_access_key_id}}"
export AWS_SECRET_ACCESS_KEY="{{.aws_secret_access_key}}"
		`

	formatTempl(sec, templ)
}

func formatTempl(sec ini.Section, templ string) {
	vals := map[string]interface{}{
		"aws_access_key_id":     sec[ACCESS_KEY_ID],
		"aws_secret_access_key": sec[SECRET_ACCESS_KEY],
	}
	t, _ := template.New("tf").Parse(strings.TrimSpace(templ))
	t.Execute(os.Stdout, vals)
	fmt.Println()

}

func unset() {
	templ := `
unset AWS_ACCESS_KEY_ID
unset AWS_SECRET_ACCESS_KEY
`
	fmt.Println(strings.TrimSpace(templ))
}

func listProfiles() {
	f := loadInifile()
	for p, _ := range f {
		fmt.Println(p)
	}
}
