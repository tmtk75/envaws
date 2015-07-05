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
	app.Usage = "awscli utils"
	app.Commands = []cli.Command{
		cli.Command{
			Name:  "env",
			Args:  "<profile>",
			Flags: []cli.Flag{},
			Action: func(ctx *cli.Context) {
				profile, _ := ctx.ArgFor("profile")
				sec := loadSection(profile)
				formatEnv(sec, ctx.String("format"))
			},
		},
		cli.Command{
			Name: "tf",
			Args: "<profile>",
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

func loadSection(profile string) ini.Section {
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
AWS_ACCESS_KEY_ID="{{.aws_access_key_id}}"
AWS_SECRET_ACCESS_KEY="{{.aws_secret_access_key}}"
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
